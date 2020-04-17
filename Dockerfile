FROM golang:1.13-alpine as gobuilder

RUN mkdir /hnh-map
WORKDIR /hnh-map

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build

FROM alpine as frontendbuilder

RUN mkdir /frontend
WORKDIR /frontend

RUN apk add --no-cache npm

COPY frontend/package.json .
RUN npm install

COPY frontend/ ./
RUN npm run build

FROM alpine

RUN mkdir /hnh-map
WORKDIR /hnh-map

COPY --from=gobuilder /hnh-map/hnh-map ./
COPY --from=frontendbuilder /frontend/dist ./frontend
COPY templates ./templates
COPY public ./public

EXPOSE 8080
CMD /hnh-map/hnh-map -grids=/map