FROM golang:1.17.6@sha256:6cc9796e7cde1ab9aad195443e65b945deb883c4b5acec4c68a99a3c9f88db34 AS builder

RUN mkdir /app
COPY go.mod go.sum /app/
WORKDIR /app
RUN go mod download

COPY . .
ARG CGO_ENABLED=0
RUN go build -o /hermit ./cmd/hermit

FROM gcr.io/distroless/static:nonroot@sha256:80c956fb0836a17a565c43a4026c9c80b2013c83bea09f74fa4da195a59b7a99
COPY --from=builder /hermit /hermit
USER nonroot:nonroot
ENTRYPOINT [ "/hermit" ]
