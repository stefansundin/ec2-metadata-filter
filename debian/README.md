```
sudo apt install -y debhelper dh-golang golang iptables
tar cvzf ../ec2-metadata-filter_1.0.0.orig.tar.gz --exclude=debian *
debuild -i -us -uc -b
```
