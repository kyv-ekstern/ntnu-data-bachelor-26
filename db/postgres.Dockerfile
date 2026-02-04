FROM postgres:18-bookworm
RUN apt-get update && apt-get -y install postgresql-18-postgis-3
CMD ["docker-entrypoint.sh", "postgres"]
