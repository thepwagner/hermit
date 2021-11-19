FROM golang:1.17.3@sha256:4f0671cf24397b6f98013bfc47882d4299573639c3714825297ddbd4b621ee92 AS builder

RUN mkdir /app
COPY go.mod go.sum /app/
WORKDIR /app
RUN go mod download

COPY . .
ARG CGO_ENABLED=0
RUN go build -o /hermit ./cmd/hermit

FROM gcr.io/distroless/static:nonroot@sha256:bca3c203cdb36f5914ab8568e4c25165643ea9b711b41a8a58b42c80a51ed609
COPY --from=builder /hermit /hermit
USER nonroot:nonroot
ENTRYPOINT [ "/hermit" ]
