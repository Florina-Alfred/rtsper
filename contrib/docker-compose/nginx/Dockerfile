FROM nginx:stable-alpine
COPY hls.conf /etc/nginx/conf.d/default.conf
# Create hls dir and give it a default index
RUN mkdir -p /usr/share/nginx/html/hls
