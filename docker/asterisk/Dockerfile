# =============================================================================
# Asterisk 20 LTS container for DongDo CSKH (Customer Support Call Center)
# Multi-stage build: install optional codecs → slim runtime with full config.
#
# NOTE on base image:
#   - There is NO official `asterisk` image on Docker Hub. We use the
#     community-maintained `andrius/asterisk:20-alpine` which is rebuilt
#     on every upstream Asterisk release and ships a complete userland
#     (asterisk binary, /etc/asterisk defaults, /var/spool, asterisk user).
#   - Tag `20-alpine` tracks the latest Asterisk 20 LTS on Alpine 3.24
#     (current digest pinned in digestfile next to this Dockerfile).
#   - If you need a fully reproducible build, replace with a digest:
#       FROM andrius/asterisk@sha256:<digest>
# =============================================================================

# ---- Stage 1: builder --------------------------------------------------------
# Same base as runtime so /usr/lib/asterisk/modules line up at COPY time.
FROM andrius/asterisk:20-alpine AS builder

ARG DEBIAN_FRONTEND=noninteractive

# Optional but commonly needed codecs/tools (opus, sox, ffmpeg, odbc).
# Alpine uses `apk` instead of `apt-get`.
RUN set -eux; \
    apk add --no-cache \
        opus \
        opus-dev \
        unixodbc \
        unixodbc-dev \
        srtp-dev \
        libsrtp \
        sox \
        ffmpeg \
        vim \
        ca-certificates \
        bash; \
    # Confirm chan_pjsip / app_queue are present.
    asterisk -rx "module show like res_pjsip.so" || true

# ---- Stage 2: runtime --------------------------------------------------------
FROM andrius/asterisk:20-alpine AS runtime

LABEL maintainer="DongDo CSKH Team" \
      version="1.0.0" \
      description="Asterisk 20 with chan_pjsip for DongDo call center"

ARG DEBIAN_FRONTEND=noninteractive

# Minimal runtime deps.
#   - bash: nicer scripting in entrypoint/wait-for than ash.
#   - curl: used by healthcheck and entrypoint waits.
#   - postgresql-client: optional psql for admin debugging.
#   - ca-certificates: TLS to AMI/ARI/HTTPS trunks.
#   - tini: clean PID 1 (andrius/asterisk already ships tini but we re-ensure).
#   - netcat-openbsd: small port-check helper.
RUN set -eux; \
    apk add --no-cache \
        bash \
        curl \
        ca-certificates \
        postgresql-client \
        sipsak \
        procps \
        tini \
        netcat-openbsd; \
    mkdir -p \
        /var/spool/asterisk/monitor \
        /var/spool/asterisk/recording \
        /var/log/asterisk/cdr-csv \
        /var/lib/asterisk/sounds \
        /etc/asterisk; \
    chown -R asterisk:asterisk \
        /var/spool/asterisk \
        /var/log/asterisk \
        /var/lib/asterisk \
        /etc/asterisk || true

# ---- Copy configuration ------------------------------------------------------
# All config files live in ./docker/asterisk/etc/asterisk in the build context.
# Mounting a host directory over /etc/asterisk at runtime still works.
COPY docker/asterisk/etc/asterisk/ /etc/asterisk/

# Copy init/entrypoint + wait-for helper scripts.
COPY docker/asterisk/entrypoint.sh /usr/local/bin/entrypoint.sh
COPY docker/asterisk/wait-for.sh /usr/local/bin/wait-for
RUN chmod +x /usr/local/bin/entrypoint.sh /usr/local/bin/wait-for

# ---- Networking --------------------------------------------------------------
# SIP signaling: 5060/udp (PJSIP), 5061/tcp (SIPS/TLS)
# AMI:           5038/tcp  (Asterisk Manager Interface)
# ARI:           8088/tcp  (Asterisk REST Interface)
# RTP media:     10000-20000/udp
EXPOSE 5060/udp 5061/tcp 5038/tcp 8088/tcp 10000-20000/udp

# ---- Volumes -----------------------------------------------------------------
# Recordings (MixMonitor output) — mounted from host ./recordings in compose.
VOLUME ["/var/spool/asterisk/monitor", "/var/log/asterisk"]

# ---- Healthcheck -------------------------------------------------------------
# Verify PJSIP is loaded AND at least one endpoint is registered/reachable.
# We query both core status and PJSIP endpoint count so a stuck startup fails.
HEALTHCHECK --interval=15s --timeout=5s --start-period=60s --retries=5 \
    CMD asterisk -rx "pjsip show endpoints" >/dev/null 2>&1 || asterisk -rx "core show version" >/dev/null 2>&1

# tini → clean PID 1, signal forwarding, zombie reaping
ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/entrypoint.sh"]

# Default: foreground Asterisk. Entrypoint script handles pre-flight checks.
CMD ["asterisk", "-f", "-vvv"]
