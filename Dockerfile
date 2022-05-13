FROM alpine AS builder
RUN echo 'nobody:x:65534:65534:nobody:/:' > /passwd

FROM scratch
COPY --from=builder /passwd /etc/passwd
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

USER nobody
COPY ratchet /ratchet
ENTRYPOINT ["/ratchet"]
