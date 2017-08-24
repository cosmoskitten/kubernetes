/home/kubernetes:
  file.directory:
    - user: root
    - group: root
    - mode: 755
    - makedirs: True

/etc/cni/net.d:
  file.directory:
    - user: root
    - group: root
    - mode: 755
    - makedirs: True

# These are all available CNI network plugins.
cni-tar:
  archive:
    - extracted
    - user: root
    - name: /home/kubernetes
    - makedirs: True
    - source: https://github.com/containernetworking/cni/releases/download/v0.6.0/cni-amd64-v0.6.0.tgz
    - tar_options: v
    - source_hash: md5=14307dc3635517923af6f3d7e12df4f7
    - archive_format: tar
    - if_missing: /home/kubernetes/bin

{% if grains['cloud'] is defined and grains.cloud in [ 'vagrant' ]  %}
# Install local CNI network plugins in a Vagrant environment
cmd-local-cni-plugins:
   cmd.run:
      - name: |
         cp -v /vagrant/cluster/network-plugins/cni/bin/* /home/kubernetes/bin/.
         chmod +x /home/kubernetes/bin/*
cmd-local-cni-config:
   cmd.run:
      - name: |
         cp -v /vagrant/cluster/network-plugins/cni/config/* /etc/cni/net.d/.
         chown root:root /etc/cni/net.d/*
         chmod 744 /etc/cni/net.d/*
{% endif -%}
