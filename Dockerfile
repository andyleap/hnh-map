FROM golang:alpine as builder

RUN mkdir /hnh-map

WORKDIR /hnh-map

ADD go.mod go.sum ./

ADD . .

RUN go build

FROM alpine

RUN mkdir /hnh-map

WORKDIR /hnh-map

COPY --from=builder hnh-map ./

COPY frontend frontend

CMD /hnh-map/hnh-map -grids=/map