FROM nginx:1.27-alpine

# Remove default config
RUN rm /etc/nginx/conf.d/default.conf

# Copy our config templates
COPY nginx.conf /etc/nginx/nginx.conf
COPY conf.d/ /etc/nginx/conf.d/
COPY docker-entrypoint.sh /docker-entrypoint.sh

# Install curl for healthcheck and envsubst for template rendering
RUN apk add --no-cache curl gettext

# Make the entrypoint executable
RUN chmod +x /docker-entrypoint.sh

HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
    CMD curl -fsS http://localhost/nginx-health || exit 1

EXPOSE 80 443
ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["nginx", "-g", "daemon off;"]
