This is a small program that you can install on EC2 instances in order to enhance the security of the EC2 metadata service.

The metadata service is used to provide temporary security credentials to the IAM role associated with an EC2 instance (among other things). The service does not have any security protections built-in, and you can find [numerous](https://blog.christophetd.fr/abusing-aws-metadata-service-using-ssrf-vulnerabilities/) [examples](http://flaws.cloud/) [online](https://news.ycombinator.com/item?id=12670316) that show how this can be exploited.

Google Compute Engine, on the other hand, [requires a special header](https://cloud.google.com/compute/docs/storing-retrieving-metadata#querying) to be present (`Metadata-Flavor: Google`). This might seems like a small thing, but it is extremely effective. [Here is a good comparison of how the different cloud metadata services behave.](https://ahmet.im/blog/comparison-of-instance-metadata-services/)

There is [a Netflix blog post](https://medium.com/netflix-techblog/netflix-information-security-preventing-credential-compromise-in-aws-41b112c15179) on the subject, and it appears that they are working with AWS to add protections based on the User-Agent header instead (the details of how and when this will be available for everyone is unclear). The benefit of checking the User-Agent header is that all SDKs should continue to just work (if you use `curl` or other libraries then you will have to update your code). I decided to support this behavior since it greatly simplifies rollout of this program since some applications will not require any modification at all.

The program acts as a reverse proxy, and relies on an iptables rule to redirect all traffic destined for 169.254.169.254 through this proxy. The program blocks any request with a User-Agent that does not start with one of the following prefixes:

```
aws-chalice/
aws-cli/
aws-sdk-
Boto3/
Botocore/
Cloud-Init/
```

In addition to whitelisting User-Agent prefixes, the program also allows requests that send the header `Metadata-Flavor: Amazon`. This can be easily added to programs such as curl.

Like GCE, the program blocks requests containing a `X-Forwarded-For` header.

Related:
- https://github.com/lyft/metadataproxy

# Install

The reverse proxy runs on port 16925 by default (you can use the `PORT` environment variable to change this), and listens only on the loopback interface.

[There is a PPA available:](https://launchpad.net/~stefansundin/+archive/ubuntu/ec2-metadata-filter)

```
sudo add-apt-repository ppa:stefansundin/ec2-metadata-filter
sudo apt-get update
sudo apt-get install ec2-metadata-filter
```

The debian package will install the program, create the user (explained below), and add a systemd service (that is started automatically). **But it will not set up the iptables rule for you.**

Run `journalctl -u ec2-metadata-filter.service` to see logs from the service.

## iptables rule

This creates a new user whose only purpose is to run the reverse proxy. Requests to 169.254.169.254 from any other user will be redirected to the proxy.

First create the user:

```
$ sudo adduser --system --no-create-home --home /nonexistent ec2-metadata
```

You could in theory use root, but that is a bad idea if security bugs are found in _this_ program, and it would also exempt root from this protection.

You can safely ignore the warning that says: `Warning: The home dir /nonexistent you specified can't be accessed: No such file or directory`

Then create the iptables rule:

```
$ sudo iptables -t nat -A OUTPUT -d 169.254.169.254 -p tcp -m owner \! --uid-owner ec2-metadata -j REDIRECT --to-port 16925
```

Then run the program as the special user:

```
$ sudo -u ec2-metadata ec2-metadata-filter
```

To persist the iptables rule, install `iptables-persistent`:

```
$ sudo apt-get install iptables-persistent
```

When it asks you if you want to save your IPv4 rules, select _Yes_.
You can also run:

```
$ sudo netfilter-persistent save
```

The file `/etc/iptables/rules.v4` should look something like the following:
```
# Generated by iptables-save v1.6.1 on Sat Feb  2 05:01:04 2019
*nat
:PREROUTING ACCEPT [0:0]
:INPUT ACCEPT [0:0]
:OUTPUT ACCEPT [6:550]
:POSTROUTING ACCEPT [6:550]
-A OUTPUT -d 169.254.169.254/32 -p tcp -m owner ! --uid-owner 113 -j REDIRECT --to-ports 16925
COMMIT
# Completed on Sat Feb  2 05:01:04 2019
# Generated by iptables-save v1.6.1 on Sat Feb  2 05:01:04 2019
*filter
:INPUT ACCEPT [101:13692]
:FORWARD ACCEPT [0:0]
:OUTPUT ACCEPT [85:28118]
COMMIT
# Completed on Sat Feb  2 05:01:04 2019
```

## Validate

Ensure that it is working properly!

Perform a request with the aws-cli (_without_ any local credentials present!):

```
$ aws sts get-caller-identity
```

In the systemd logs, you should see the following:

```
Proxying request to /latest/meta-data/iam/security-credentials/ from User-Agent: aws-cli/1.15.71 Python/3.5.2 Linux/4.15.0-43-generic botocore/1.10.70
```

That means that the request was received by the program which then forwarded it after checking the User-Agent header. Now try with curl:

```
$ curl -i http://169.254.169.254/latest/meta-data/iam/security-credentials/
HTTP/1.1 400 Bad Request
```

The request was blocked, great!

Now try adding the `Metadata-Flavor: Amazon` header:

```
$ curl -i -H 'Metadata-Flavor: Amazon' http://169.254.169.254/latest/meta-data/iam/security-credentials/
HTTP/1.1 200 OK
```

That worked!

# Troubleshooting

Print your iptables rules by running `sudo iptables-save`. Does it contain the nat rule to redirect traffic destined for 169.254.169.254?

If you see the error `http: proxy error: context canceled`, that means that the program is having problems forwarding the request to the real metadata service. Are you running on an EC2 instance?

If you see hundreds of lines that eventually end with `http: proxy error: dial tcp 169.254.169.254:80: socket: too many open files`, that means that the program is also affected by the iptables rule. Are you running the program as the special user?

Elastic Beanstalk issue requests to the metadata service using `curl`, so it will not work out of the box. This requires more research.

To undo the iptables rule, run `sudo iptables -t nat -F`. This will flush all rules in the nat table.
