FROM golang:1.22

WORKDIR /app

COPY . .

RUN go mod tidy

EXPOSE 9000

CMD ["go", "run", "."]
