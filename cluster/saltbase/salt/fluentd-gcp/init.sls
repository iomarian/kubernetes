/etc/kubernetes/manifests/fluentd-gcp.json:
  file.managed:
    - source: salt://fluentd-gcp/fluentd-gcp.json
    - user: root
    - group: root
    - mode: 644
    - makedirs: true
    - dir_mode: 755