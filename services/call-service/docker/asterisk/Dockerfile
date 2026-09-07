FROM alpine:3.19

RUN apk add --no-cache \
    asterisk \
    asterisk-sample-config \
    asterisk-sound-en \
    openssl \
    ca-certificates

RUN mkdir -p /etc/asterisk/keys && \
    openssl req -new -x509 -days 365 -nodes \
    -out /etc/asterisk/keys/asterisk.pem \
    -keyout /etc/asterisk/keys/asterisk.pem \
    -subj "/C=VN/ST=Hanoi/L=Hanoi/O=DongDoCS/OU=Voice/CN=asterisk"

COPY http.conf /etc/asterisk/http.conf
COPY ari.conf /etc/asterisk/ari.conf
COPY pjsip.conf /etc/asterisk/pjsip.conf
COPY extensions.conf /etc/asterisk/extensions.conf
COPY rtp.conf /etc/asterisk/rtp.conf

EXPOSE 8088 5060/udp 10000-10200/udp

CMD ["asterisk", "-f", "-U", "asterisk", "-vvvg"]
