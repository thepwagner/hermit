FROM golang:1.17.1@sha256:a992e99cf4843d8dd2ecab47b3532110d7f60a437d49dba6049941f31c33fe20 AS builder

RUN mkdir /app
COPY go.mod go.sum /app/
WORKDIR /app
RUN go mod download

COPY . .
ARG CGO_ENABLED=0
RUN go build -o /hermit ./cmd/hermit

FROM gcr.io/distroless/static:nonroot@sha256:07869abb445859465749913267a8c7b3b02dc4236fbc896e29ae859e4b360851
COPY --from=builder /hermit /hermit
USER nonroot:nonroot
ENTRYPOINT [ "/hermit" ]
