FROM nginx:stable-alpine
LABEL image=dggstats
COPY out/index.html .
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]