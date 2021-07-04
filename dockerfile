FROM golang:alpine as builder
LABEL stage=builder
LABEL image=dggstats
WORKDIR /app
COPY . .
RUN apk add --update bash coreutils perl jq curl sed && rm -rf /var/cache/apk/*
RUN curl -s 'https://cdn.destiny.gg/emotes/emotes.json' | jq -r '.[].prefix' | paste -sd" " - | ( read emotes; sed "s/ALOTOFEMOTES/$emotes/" pisg.cfg.initial > pisg.cfg )
RUN mkdir -p logs/; go run ./main.go $(date -uI --date='-31 days') $(date -uI --date='-1 days') logs/
RUN mkdir -p cache/ out/; bash -c "perl ./pisg/pisg logs/"
RUN cp out/index.html index.html

FROM nginx:stable-alpine
LABEL image=dggstats
COPY --from=builder /app/ /usr/share/nginx/html/
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]