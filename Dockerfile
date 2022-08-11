FROM --platform=$BUILDPLATFORM alpine AS builder

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Normally we would set this to run as "nobody", but to play nicely with GitHub
# Actions, it must run as the default user:
#
#   https://docs.github.com/en/actions/creating-actions/dockerfile-support-for-github-actions#user
#
# USER nobody

COPY ratchet /ratchet
ENTRYPOINT ["/ratchet"]
