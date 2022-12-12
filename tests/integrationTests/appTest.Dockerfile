FROM golang:1.19 as builder

ADD ./appTest.go ./

ENTRYPOINT [ "/appTest" ]

EXPOSE 8080