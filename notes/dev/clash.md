# How to install clash on POSIX system with command line.

## Pre-request: You need to know which executable your distro can run without complaining.
1. Go to https://github.com/Dreamacro/clash/releases to find the latest or the version you desired.
2. Find the correct distro version, for example, if you are running a distro with arm64 archtecture, you should download
   the one with `armv8` suffix.
3. Download the executables with curl or wget at your wish.

```shell
$ curl -O [url/to/clash]
```

if you downloaded the compressed version you need to do:

```shell
$ sudo mkdir /opt/clash
$ sudo gunzip [the/zip/file] /opt/clash
```

4. After unzip the file, you will need to download the config file from your subscription, and the `Country.mmdb` file.

```shell
$ wget -O config.yaml [ Subscription Links ]
$ wget -O Country.mmdb https://www.sub-speeder.com/client-download/Country.mmdb
```

From now on, everything is sattled.

You can do:

```shell
$ cd /opt/clash
$ sudo chmod +x ./clash
$ sudo clash -d .
```

Then the service would be up an runinng.

## Start the service after boot

Using linux services control to start the service after reboot

```shell
$ sudo touch /etc/systemd/system/clash.service
$ sudo vi /etc/systemd/system/clash.service
```

Copy this file into your `/etc/systemd/system/clash.service`

```config
[Unit]
Description=clash daemon

[Service]
Type=simple
User=root
ExecStart=/opt/clash/clash -d /opt/clash/
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

Save the editted file by pressing `ESC` and `Shift + ZZ` or `:wq`

```shell
$ sudo systemctl daemon-reload
$ sudo systemctl start clash.service
$ sudo systemctl enable clash.service
```

- If you want to restart your service you can do:

```shell
$ sudo systemctl restart clash.service
```

- If you want to check the status of your service you can do:

```shell
$ systemctl status clash
```

If you want to constantly update your clash subscription, you should preceed to this next chapter:

## Using cron job to refresh the subscription

TODO