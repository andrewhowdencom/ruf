# build stage
FROM golang:1.25-alpine as builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /ruf .

# final stage
FROM scratch

COPY --from=builder /ruf /ruf

ENTRYPOINT ["/ruf", "run"]
