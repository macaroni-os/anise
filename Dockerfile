FROM macaronios/terragon-minimal:latest-amd64 as builder
RUN anise i -y --sync-repos make upx-bin go ca-certificates git && \
      anise cleanup --purge-repos
ADD . /anise
RUN cd /anise && make build-small

FROM scratch
ENV ANISE_NOLOCK=true
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /anise/anise /usr/bin/anise

ENTRYPOINT ["/usr/bin/anise"]
