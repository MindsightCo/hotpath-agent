FROM golang:alpine
WORKDIR /go/src/github.com/MindsightCo/hotpath-agent
COPY . .

RUN go vet ./...
RUN go test ./...
RUN go install -v

ENV API api:8080
ENV PROTOCOL http
ENV MINDSIGHT_CLIENT_ID=change-me
ENV MINDSIGHT_CLIENT_SECRET=change-me

CMD hotpath-agent -server ${PROTOCOL}://${API}/samples/
