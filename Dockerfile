FROM golang:latest as builder
WORKDIR /go/src/github.com/MindsightCo/hotpath-agent
COPY . .

RUN go vet ./...
RUN go test ./...
RUN CGO_ENABLED=0 go install -v

from alpine:latest

RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*

WORKDIR /root
COPY --from=builder /go/bin/hotpath-agent ./

EXPOSE 8000

ENV MINDSIGHT_CLIENT_ID=change-me
ENV MINDSIGHT_CLIENT_SECRET=change-me

CMD ./hotpath-agent ${API:+-server ${PROTOCOL}://${API}/query}
