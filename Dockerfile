FROM golang:alpine as downloader-builder
LABEL stage=downloader-builder
LABEL image=dggstats-build
WORKDIR /app
COPY main.go .
RUN apk add git
RUN git clone https://github.com/vyneer/pisg
RUN GOOS=linux GARCH=amd64 CGO_ENABLED=0 go build -o downloader -v main.go

FROM perl:slim-threaded-bullseye as builder
LABEL stage=builder
LABEL image=dggstats
WORKDIR /app
RUN cpanm URI::Find::Schemeless
COPY --from=downloader-builder /app/downloader .
COPY --from=downloader-builder /app/pisg ./pisg
COPY ./cache ./cache
COPY pisg.cfg.initial .
RUN mkdir -p logs/; ./downloader $(date -uI --date='-31 days') $(date -uI --date='-1 days') logs/
RUN mkdir -p cache/ out/; perl ./pisg/pisg logs/
RUN cp out/index.html index.html

FROM nginx:stable-alpine
LABEL image=dggstats
COPY --from=builder /app/ /usr/share/nginx/html/
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]