FROM alpine AS build-env
RUN apk add --update --no-cache mailcap

FROM gcr.io/distroless/static-debian12:debug-nonroot
ARG TARGETPLATFORM
LABEL maintainer="The Perses Authors <perses-team@googlegroups.com>"

COPY  ${TARGETPLATFORM}/perses-mcp-server                  /bin/perses-mcp-server
COPY  LICENSE                                              /LICENSE
COPY --from=build-env                                      /etc/mime.types /etc/mime.types

EXPOSE     8080
ENTRYPOINT ["/bin/perses-mcp-server"]
