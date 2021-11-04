FROM golang:1.17.3@sha256:dac1fd50dc298852005ed2d84baa4f15ca86b1042fce6d8da3f98d6074294bf4 AS builder

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
