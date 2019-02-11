To install on a systemd system:

```
sudo cp stuff-org /etc/init.d/
# edit paths at top of stuff-org script as necessary
chmod 755 /etc/init.d/stuff-org
sudo cp stuff-org.service /etc/systemd/user/
sudo systemctl enable stuff-org
```

