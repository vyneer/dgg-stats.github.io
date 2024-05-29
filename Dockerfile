FROM lipanski/docker-static-website:latest
LABEL image=dggstats
COPY out/index.html .
