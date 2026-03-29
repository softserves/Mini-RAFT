FROM golang:1.22

WORKDIR /app

COPY . .

RUN go mod tidy

EXPOSE 9002

CMD ["go", "run", "main.go"]
