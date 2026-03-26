FROM golang:1.21-alpine

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

RUN go build -o frontend .

EXPOSE 3000

CMD ["./frontend"]