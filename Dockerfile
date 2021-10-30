FROM golang:1.17.2@sha256:124966f5d54a41317ee81ccfe5f849d4f0deef4ed3c5c32c20be855c51c15027 AS builder

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
