package service

import (
	"bytes"
	"context"
	"deeplx-local/domain"
	"deeplx-local/pkg"
	"github.com/imroc/req/v3"
	lop "github.com/samber/lo/parallel"
	"github.com/sourcegraph/conc/pool"
	"github.com/sourcegraph/conc/stream"
	"log"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
)

const (
	maxLength           = 4096
	maxFailures         = 3 //最大健康检查错误次数
	healthCheckInterval = time.Minute
)

var sentenceRe = regexp.MustCompile(`[^.!?。！？]+[.!?。！？]`) //还有一种方式是 [^.!?。！？\s]+[.!?。！？]?\s* 这样能分割得更细小，但感觉没必要

type TranslateService interface {
	GetTranslateData(trReq domain.TranslateRequest) domain.TranslateResponse
}

type Server struct {
	URL          string
	isAvailable  bool
	failureCount int
}

type LoadBalancer struct {
	Servers            []*Server
	client             *req.Client
	index              uint32
	unavailableServers []*Server    // 不可用的服务器
	healthCheck        *time.Ticker // 健康检查定时器
}

// NewLoadBalancer 负载均衡
func NewLoadBalancer(vlist *[]string) TranslateService {
	servers := lop.Map(*vlist, func(item string, index int) *Server {
		return &Server{URL: item, isAvailable: true}
	})
	lb := &LoadBalancer{
		Servers:            servers,
		client:             req.NewClient().SetTimeout(3 * time.Second),
		unavailableServers: make([]*Server, 0),
		healthCheck:        time.NewTicker(healthCheckInterval),
	}
	go lb.startHealthCheck() // 开启定时健康检查
	return lb
}

func (lb *LoadBalancer) GetTranslateData(trReq domain.TranslateRequest) domain.TranslateResponse {
	text := trReq.Text
	textLength := len(text)

	if textLength <= maxLength {
		return lb.sendRequest(trReq)
	}

	var textParts []string
	var currentPart bytes.Buffer

	sentences := sentenceRe.FindAllString(text, -1)

	for _, sentence := range sentences {
		if currentPart.Len()+len(sentence) <= maxLength {
			currentPart.WriteString(sentence)
		} else {
			textParts = append(textParts, currentPart.String())
			currentPart.Reset()
			currentPart.WriteString(sentence)
		}
	}

	if currentPart.Len() > 0 {
		textParts = append(textParts, currentPart.String())
	}

	var results = make([]string, 0, len(textParts))
	s := stream.New()

	for _, part := range textParts {
		s.Go(func() stream.Callback {
			res := lb.sendRequest(domain.TranslateRequest{
				Text:       part,
				SourceLang: trReq.SourceLang,
				TargetLang: trReq.TargetLang,
			})
			return func() {
				results = append(results, res.Data)
			}
		})
	}

	s.Wait()

	return domain.TranslateResponse{
		Code: 200,
		Data: strings.Join(results, ""),
	}
}

func (lb *LoadBalancer) sendRequest(trReq domain.TranslateRequest) domain.TranslateResponse {
    maxRetries := 3
    for retry := 0; retry < maxRetries; retry++ {
        ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*5)
        defer cancelFunc()
        resultChan := make(chan domain.TranslateResponse, 1)

        contextPool := pool.New().WithContext(ctx).WithMaxGoroutines(min(len(lb.Servers), 5))
        for i := 0; i < 5; i++ {
            contextPool.Go(func(ctx context.Context) error {
                server := lb.getServer()
                var trResult domain.TranslateResponse
                response, err := lb.client.R().
                    SetContext(ctx).
                    SetBody(trReq).
                    SetSuccessResult(&trResult).
                    Post(server.URL)

                if err != nil {
                    return err
                }
                response.Body.Close()

                if trResult.Code == 200 && len(trResult.Data) > 0 {
                    select {
                    case resultChan <- trResult:
                        cancelFunc()
                    case <-ctx.Done():
                        return nil
                    default:
                    }
                } else {
                    server.isAvailable = false
                    lb.unavailableServers = append(lb.unavailableServers, server)
                }
                return nil
            })
        }

        select {
        case result := <-resultChan:
            return result
        case <-ctx.Done():
            // 如果所有请求都失败，继续重试
            continue
        }
    }

    // 如果所有重试都失败，返回空响应
    return domain.TranslateResponse{}
}


func (lb *LoadBalancer) getServer() *Server {
	index := atomic.AddUint32(&lb.index, 1) - 1
	server := lb.Servers[index%uint32(len(lb.Servers))]

	for !server.isAvailable {
		index = atomic.AddUint32(&lb.index, 1) - 1
		server = lb.Servers[index%uint32(len(lb.Servers))]
	}
	return server
}

func (lb *LoadBalancer) startHealthCheck() {
	for {
		select {
		case <-lb.healthCheck.C:
			for i := 0; i < len(lb.unavailableServers); i++ {
				server := lb.unavailableServers[i]
				flag, _ := pkg.CheckURLAvailability(lb.client, server.URL)
				if flag {
					server.isAvailable = true
					server.failureCount = 0
					copy(lb.unavailableServers[i:], lb.unavailableServers[i+1:])
					lb.unavailableServers = lb.unavailableServers[:len(lb.unavailableServers)-1]
					i--
					log.Printf("Server %s is available now", server.URL)
				} else {
					server.failureCount++
					if server.failureCount >= maxFailures {
						copy(lb.unavailableServers[i:], lb.unavailableServers[i+1:])
						lb.unavailableServers = lb.unavailableServers[:len(lb.unavailableServers)-1]
						i--
						log.Printf("Server %s is removed due to max failures", server.URL)
					}
				}
			}
		}
	}
}
