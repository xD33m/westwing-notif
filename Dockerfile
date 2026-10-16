# The binary is cross-compiled by deploy.sh (GOARCH=arm64, static, with CA
# roots and tzdata embedded), so the image is just that one file.
FROM scratch
COPY westwing-notif /westwing-notif
ENTRYPOINT ["/westwing-notif"]
CMD ["run"]
