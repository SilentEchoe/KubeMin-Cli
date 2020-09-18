FROM golang:latest

WORKDIR $GOPATH/src/github.com/EDDYCJY/LearningNotes-GoMicro
COPY . $GOPATH/src/github.com/EDDYCJY/LearningNotes-GoMicro
RUN go build .

EXPOSE 8000
ENTRYPOINT ["./main"]