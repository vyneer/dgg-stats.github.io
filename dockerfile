FROM node:current-alpine as builder
WORKDIR /app
COPY . .
RUN apk add --update coreutils perl && rm -rf /var/cache/apk/*
RUN npm ci
RUN mkdir -p logs/; npm run pull -- $(date -uI --date='-31 days') $(date -uI --date='-1 days') -o logs/
RUN mkdir -p cache/ out/; perl ./pisg/pisg logs/
RUN npm run minify

FROM nginx:stable-alpine
COPY --from=builder /app/ /usr/share/nginx/html/
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]