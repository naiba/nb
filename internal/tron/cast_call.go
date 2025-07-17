package tron

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/naiba/nb/internal"
)

func CastCall(rpc string, overrideCode []string, otherArgs []string) error {
	var overrideCodeMap sync.Map
	if len(overrideCode) > 0 {
		for _, arg := range overrideCode {
			parts := strings.Split(arg, ":")
			if len(parts) == 2 {
				overrideCodeMap.Store(strings.ToLower(parts[0]), parts[1])
				continue
			}
			return fmt.Errorf("invalid override code: %s", arg)
		}
	}

	targetURL, err := url.Parse(rpc)
	if err != nil {
		return fmt.Errorf("invalid RPC URL: %v", err)
	}

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return fmt.Errorf("failed to create listener: %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port

	handler := &proxyHandler{
		target:          targetURL,
		overrideCodeMap: &overrideCodeMap,
	}

	server := &http.Server{
		Handler: handler,
	}

	var finalError error
	closeCh := make(chan struct{})
	var closeOnce sync.Once

	go func() {
		err := server.Serve(listener)
		if err != nil {
			finalError = err
		}
		closeOnce.Do(func() { close(closeCh) })
	}()

	time.Sleep(time.Second)

	go func() {
		finalError = internal.ExecuteInHost(nil, "cast", append([]string{
			"call",
			"--rpc-url",
			fmt.Sprintf("http://localhost:%d%s", port, targetURL.Path),
		}, otherArgs...)...)
		closeOnce.Do(func() { close(closeCh) })
	}()

	<-closeCh
	return finalError
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
	ID      interface{} `json:"id"`
}

type proxyHandler struct {
	target          *url.URL
	overrideCodeMap *sync.Map
}

func (p *proxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to read request body: %v", err), http.StatusBadRequest)
		return
	}
	r.Body.Close()

	var mockResponse *JSONRPCResponse
	if body, mockResponse = p.getMockResponse(body); mockResponse != nil {
		p.sendJSONResponse(w, mockResponse)
		return
	}

	p.forwardRequestWithBody(w, r, body)
}

func (p *proxyHandler) getMockResponse(req []byte) ([]byte, *JSONRPCResponse) {
	switch gjson.GetBytes(req, "method").String() {
	case "eth_getTransactionCount":
		return req, &JSONRPCResponse{
			JSONRPC: gjson.GetBytes(req, "jsonrpc").String(),
			Result:  "0x1b4",
			ID:      gjson.GetBytes(req, "id").Value(),
		}
	case "eth_call":
		req, _ = sjson.DeleteBytes(req, "params.0.input")
		req, _ = sjson.DeleteBytes(req, "params.0.chainId")
		return req, nil
	case "eth_getCode":
		contractAddr := strings.ToLower(gjson.GetBytes(req, "params.0").String())
		overrideCode, ok := p.overrideCodeMap.Load(contractAddr)
		if ok {
			var resp JSONRPCResponse
			resp.JSONRPC = gjson.GetBytes(req, "jsonrpc").String()
			resp.Result = overrideCode
			resp.ID = gjson.GetBytes(req, "id").Value()
			return req, &resp
		}
		req, _ = sjson.DeleteBytes(req, "params.-1")
		req, _ = sjson.SetBytes(req, "params.-1", "latest")
		return req, nil
	case "eth_getBalance", "eth_getStorageAt":
		req, _ = sjson.DeleteBytes(req, "params.-1")
		req, _ = sjson.SetBytes(req, "params.-1", "latest")
		return req, nil
	default:
		return req, nil
	}
}

func (p *proxyHandler) sendJSONResponse(w http.ResponseWriter, response *JSONRPCResponse) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

func (p *proxyHandler) forwardRequestWithBody(w http.ResponseWriter, r *http.Request, body []byte) {
	targetURL := *p.target
	targetURL.Path = r.URL.Path
	targetURL.RawQuery = r.URL.RawQuery

	proxyReq, err := http.NewRequest(r.Method, targetURL.String(), bytes.NewReader(body))
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to create proxy request: %v", err), http.StatusInternalServerError)
		return
	}

	for name, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(name, value)
		}
	}

	proxyReq.Host = p.target.Host

	client := &http.Client{}
	resp, err := client.Do(proxyReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("proxy request failed: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for name, values := range resp.Header {
		for _, value := range values {
			if name == "Content-Length" {
				continue
			}
			w.Header().Add(name, value)
		}
	}

	w.WriteHeader(resp.StatusCode)

	var respBody bytes.Buffer

	if _, err := io.Copy(&respBody, resp.Body); err != nil {
		log.Printf("failed to copy response body: %v", err)
	}

	method := gjson.GetBytes(body, "method").String()

	switch method {
	case "eth_getBlockByNumber":
		respBodyStr := respBody.String()
		respBodyStr, _ = sjson.Set(respBodyStr, "result.stateRoot", getRndHash())
		respBodyStr, _ = sjson.Set(respBodyStr, "result.requestsHash", getRndHash())
		respBodyStr, _ = sjson.Set(respBodyStr, "result.withdrawals", []struct{}{})
		respBodyStr, _ = sjson.Set(respBodyStr, "result.withdrawalsRoot", getRndHash())
		respBodyStr, _ = sjson.Set(respBodyStr, "result.blobGasUsed", "0x0")
		respBodyStr, _ = sjson.Set(respBodyStr, "result.excessBlobGas", "0x0")
		respBodyStr, _ = sjson.Set(respBodyStr, "result.milliTimestamp", "0x197c36ef8d2")
		respBodyStr, _ = sjson.Set(respBodyStr, "result.parentBeaconBlockRoot", getRndHash())
		respBody.Reset()
		respBody.WriteString(respBodyStr)
	}

	w.Write(respBody.Bytes())
}

func getRndHash() string {
	var rndBytes [32]byte
	rand.Read(rndBytes[:])
	return fmt.Sprintf("0x%x", rndBytes)
}
