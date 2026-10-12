# One multi-stage project image builds every cmd/* process binary
# (CLAUDE.md §3: Docker Compose is the default deployment mechanism, but
# the sensor/actuator/decision processes must remain independently
# deployable/restartable — compose.yaml runs this same image once per
# service with a different `command:`, not one merged process).
#
# Mirrors docs/internal/D7065E/buildingsim/Dockerfile's own
# builder-then-minimal-runtime shape (course-provided convention), reused
# rather than invented from scratch.
FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
COPY config/ config/

RUN mkdir -p /out && \
    for cmd in simulator eventdata decision actuator cardreader occupancysensor doorstatesensor dashboard; do \
      CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "/out/${cmd}" "./cmd/${cmd}"; \
    done

# alpine (not scratch): this is a course project where operators are
# expected to be able to `docker compose exec <service> sh` to inspect a
# stuck process — a small, explainable convenience, not extra
# architecture (CLAUDE.md §21 oral exam constraint).
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /out/ /app/bin/
COPY config/ /app/config/
WORKDIR /app
ENV PATH="/app/bin:${PATH}"
