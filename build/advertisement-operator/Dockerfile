FROM golang:1.14 as builder
ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go
COPY . /go/src/github.com/liqotech/liqo
WORKDIR /go/src/github.com/liqotech/liqo
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/advertisement-operator/
RUN cp advertisement-operator /usr/bin/advertisement-operator

FROM scratch
COPY --from=builder /usr/bin/advertisement-operator /usr/bin/advertisement-operator
ENTRYPOINT [ "/usr/bin/advertisement-operator" ]