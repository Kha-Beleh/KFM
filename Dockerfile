FROM busybox:1.37.0 AS artifact
WORKDIR /artifact
COPY dist/kfm dist/kfm.sha256 ./
RUN sha256sum -c kfm.sha256 && chmod 0555 kfm

FROM busybox:1.37.0
COPY --from=artifact /artifact/kfm /usr/local/bin/kfm
RUN mkdir /work && chown 10001:10001 /work
USER 10001:10001
WORKDIR /work
ENTRYPOINT ["/usr/local/bin/kfm"]
