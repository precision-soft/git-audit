package service

import (
    nethttp "net/http"

    httpclientcontract "github.com/precision-soft/melody/v3/httpclient/contract"
)

/* fakeResponse is a minimal httpclientcontract.Response used to drive parsing tests without network access. */
type fakeResponse struct {
    statusCode int
    body       []byte
}

func (instance *fakeResponse) StatusCode() int {
    return instance.statusCode
}

func (instance *fakeResponse) Status() string {
    return ""
}

func (instance *fakeResponse) Headers() nethttp.Header {
    return nil
}

func (instance *fakeResponse) Body() []byte {
    return instance.body
}

func (instance *fakeResponse) Request() *nethttp.Request {
    return nil
}

func (instance *fakeResponse) Json(any) error {
    return nil
}

func (instance *fakeResponse) String() string {
    return string(instance.body)
}

func (instance *fakeResponse) IsSuccess() bool {
    return instance.statusCode >= 200 && instance.statusCode < 300
}

func (instance *fakeResponse) IsClientError() bool {
    return instance.statusCode >= 400 && instance.statusCode < 500
}

func (instance *fakeResponse) IsServerError() bool {
    return instance.statusCode >= 500 && instance.statusCode < 600
}

/* fakeHttpClient returns a canned response/error for every request, regardless of method or url. */
type fakeHttpClient struct {
    response httpclientcontract.Response
    err      error
}

func (instance *fakeHttpClient) Get(string, ...httpclientcontract.RequestOption) (httpclientcontract.Response, error) {
    return instance.response, instance.err
}

func (instance *fakeHttpClient) Post(string, any, ...httpclientcontract.RequestOption) (httpclientcontract.Response, error) {
    return instance.response, instance.err
}

func (instance *fakeHttpClient) Put(string, any, ...httpclientcontract.RequestOption) (httpclientcontract.Response, error) {
    return instance.response, instance.err
}

func (instance *fakeHttpClient) Patch(string, any, ...httpclientcontract.RequestOption) (httpclientcontract.Response, error) {
    return instance.response, instance.err
}

func (instance *fakeHttpClient) Delete(string, ...httpclientcontract.RequestOption) (httpclientcontract.Response, error) {
    return instance.response, instance.err
}

func (instance *fakeHttpClient) Request(string, string, ...httpclientcontract.RequestOption) (httpclientcontract.Response, error) {
    return instance.response, instance.err
}

func (instance *fakeHttpClient) RequestStream(string, string, ...httpclientcontract.RequestOption) (httpclientcontract.StreamResponse, error) {
    return nil, nil
}
