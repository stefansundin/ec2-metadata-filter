```
sudo apt install -y debhelper dh-golang golang iptables
git clean -fdX
tar cvJf ../ec2-metadata-filter_1.0.0.orig.tar.xz --exclude=debian *
debuild -i -us -uc -b
```
