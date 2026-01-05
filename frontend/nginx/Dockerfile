# syntax=docker/dockerfile:1

FROM alpine:3.20 AS examples_index

RUN apk add --no-cache bash coreutils findutils grep sed

WORKDIR /src

COPY scripts/generate_examples_index.sh /src/scripts/generate_examples_index.sh
COPY examples /src/examples

RUN /src/scripts/generate_examples_index.sh /src/examples /src/examples/index.json


FROM nginxinc/nginx-unprivileged:alpine

COPY frontend/nginx/nginx.conf /etc/nginx/conf.d/default.conf
COPY frontend/nginx/docs/ /usr/share/nginx/html/

# Only expose .html templates + the generated index.json. No directory listing.
COPY --from=examples_index /src/examples/*.html /usr/share/nginx/html/examples/
COPY --from=examples_index /src/examples/index.json /usr/share/nginx/html/examples/index.json
