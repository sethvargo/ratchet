FROM cgr.dev/chainguard/static:latest-20230102
# Normally we would set this to run as "nobody", but to play nicely with GitHub
# Actions, it must run as the default user:
#
#   https://docs.github.com/en/actions/creating-actions/dockerfile-support-for-github-actions#user
#
# USER nobody

COPY ratchet /ratchet
ENTRYPOINT ["/ratchet"]
