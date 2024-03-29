FROM golang:1.20-alpine

RUN adduser -S go
USER go
WORKDIR /home/go/app

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY *.go .
COPY sources.txt .

RUN go build -o DiscordTechNewsNotifier

CMD [ "./DiscordTechNewsNotifier" ]
