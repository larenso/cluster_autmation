#   Official Postgres 19.0 (Debian 13 / trixie) + pgvector + PGSTY vchord 1.5.0.
#   pgsty/vchord 1.5.0 "supports PostgreSQL 15-19 on x86_64 and aarch64"
#     https://github.com/pgsty/vchord/releases/tag/1.5.0
#     deb Depends: postgresql-19, postgresql-19-pgvector, libgcc-s1, libc6 (>= 2.35)
#   pgvector is still required (vchord.control: requires = 'vector'; the `vector` type is pgvector's),
#   so we build pgvector from source (pinned tag) alongside the vchord deb.
ARG PG_BASE=postgres:19.0-trixie

FROM ${PG_BASE} AS ext-build
ARG VCHORD_VERSION=1.5.0
ARG PGVECTOR_VERSION=0.8.5
RUN set -euo pipefail; \
    apt-get update && apt-get install -y --no-install-recommends \
      build-essential postgresql-server-dev-19 curl ca-certificates; \
    rm -rf /var/lib/apt/lists/*; \
    ARCH="$(dpkg --print-architecture)"; \
    case "$ARCH" in \
      arm64) SHA=53d31a55d7591131772fd930507e2cd95c6041a352fd2e37846cf50c76740751 ;; \
      amd64) SHA=ae2c40877942f4882c7ebb7ef02553e6f2c83d5800ae98bc8a9bdc92342edf9e ;; \
      *) echo "unsupported architecture: $ARCH" >&2; exit 1 ;; \
    esac; \
    curl -sL --retry 3 -o /tmp/vchord.deb \
      "https://github.com/pgsty/vchord/releases/download/${VCHORD_VERSION}/postgresql-19-vchord_${VCHORD_VERSION}-1_${ARCH}.deb"; \
    echo "${SHA}  /tmp/vchord.deb" | sha256sum -c -; \
    dpkg-deb -x /tmp/vchord.deb /vchord-stage; \
    curl -sL "https://github.com/pgvector/pgvector/archive/refs/tags/v${PGVECTOR_VERSION}.tar.gz" | tar xz -C /usr/src; \
    cd "/usr/src/pgvector-${PGVECTOR_VERSION}"; \
    make -j"$(nproc)"; \
    make install DESTDIR=/pgvector-stage

FROM ${PG_BASE}
COPY --from=ext-build /pgvector-stage/usr /usr
COPY --from=ext-build /vchord-stage/usr /usr
# TRIPWIRE: fail the build loudly if artifacts ever land in the wrong pg tree
RUN test -f /usr/lib/postgresql/19/lib/vchord.so \
 && test -f /usr/share/postgresql/19/extension/vchord.control \
 && test -f /usr/share/postgresql/19/extension/vchord--1.5.0.sql \
 && test -f /usr/share/postgresql/19/extension/vector.control \
 && echo "TRIPWIRE OK: pgsty vchord 1.5.0 + pgvector artifacts in pg19 tree"
