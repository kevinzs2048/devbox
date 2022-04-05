import paramiko
import random
import string
import os
import json
import subprocess
from os.path import exists
import const
import time
from datetime import datetime


class Provisioner(object):
    def __init__(self, test_type):
        self.test_type = test_type
        self.node_map = None
        self.tf_conf_dir = None
        self.node_ip_list = []
        self.user = const.DEFAULT_SSH_USER

    def _debug(self, msg, *args):
        """_"""
        self.logger.debug(msg, *args)

    def _error(self, msg, *args):
        """_"""
        self.logger.error(msg, *args)

    def host_name_gen(self):
        # Generate 8-bit strings from a-zA-Z0-9
        return ''.join(random.sample(string.ascii_letters + string.digits, 8))

    def ssh_connection(self, ip):
        private_key = paramiko.RSAKey.from_private_key_file(const.SSH_PRIVATE_KEY)
        ssh_client = paramiko.SSHClient()
        ssh_client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
        ssh_client.connect(hostname=ip, port=22, username=self.ssh_user, pkey=private_key)

        # Test SSH connection
        stdin, stdout, stderr = ssh_client.exec_command('ls /')
        if stderr:
            self._error(stderr.read().decode('utf-8'))
            return
        self._debug("SSH client for IP: " + ip + " initialization is finished")
        return ssh_client

    def ssh_close(self, ssh_client):
        ssh_client.close()

    def ssh_exec(self, ssh_client, cmd):
        # Test SSH connection
        stdin, stdout, stderr = ssh_client.exec_command(cmd)
        if stderr:
            result = stderr.read()
            self._debug(result.decode('utf-8'))
            return
        return stdout

    def copy_dir(self, test_name):
        tf_conf_dir = const.TERRAFORM_CONF_DIR + test_name
        source_dir = const.TERRAFORM_CONF_TEMPLATE_DIR
        if not os.path.exists(tf_conf_dir):
            try:
                os.mkdir(tf_conf_dir)
            except OSError:
                self._error("mkdir failed: " + tf_conf_dir)
        for f in os.listdir(source_dir):
            source_file = os.path.join(source_dir, f)
            target_file = os.path.join(tf_conf_dir, f)
            if not os.path.exists(target_file) or (
                    os.path.exists(target_file) and (
                    os.path.getsize(target_file) != os.path.getsize(source_file))):
                open(target_file, "wb").write(open(source_file, "rb").read())

    def prepare_tf_conf(self):
        test_hash = self.host_name_gen()
        test_name = "lustre-" + test_hash
        self.copy_dir(test_name)
        self.tf_conf_dir = const.TERRAFORM_CONF_DIR + test_name

        network_port_prefix = "lustre_" + test_hash
        tf_vars = {
            "node01": test_name + "-01",
            "node02": test_name + "-02",
            "node03": test_name + "-03",
            "node04": test_name + "-04",
            "node05": test_name + "-05",
            "lustre_client01_port": network_port_prefix + "_client01_port",
            "lustre_client02_port": network_port_prefix + "_client02_port",
            "lustre_mds01_port": network_port_prefix + "_mds01_port",
            "lustre_mds02_port": network_port_prefix + "_mds02_port",
            "lustre_ost01_port": network_port_prefix + "_ost01_port"
        }
        with open(self.tf_conf_dir + const.TERRAFORM_VARIABLES_JSON, "w") as f:
            json.dump(tf_vars, f)

    def terraform_init(self):
        os.chdir(self.tf_conf_dir)
        if os.path.exists(const.TERRAFORM_VARIABLES_JSON):
            try:
                result = subprocess.check_output([const.TERRAFORM_BIN, 'init'])
            except subprocess.CalledProcessError as e:
                #result = e.output  # Output generated before error
                #code = e.returncode  # Return code
                self._error("Error when terraform_init: " + e.output)
                return False
            self._debug(result.decode('utf-8'))
            return True
        return False

    def terraform_apply(self):
        os.chdir(self.tf_conf_dir)
        if self.terraform_init():
            try:
                result = subprocess.check_output([const.TERRAFORM_BIN, 'apply -auto-approve'])
            except subprocess.CalledProcessError as e:
                #result = e.output  # Output generated before error
                #code = e.returncode  # Return code
                self._error("Error when terraform_apply: " + e.output)
                return False
            self._debug(result.decode('utf-8'))
            return True
        return False

    def write_to_file(self):
        result = subprocess.check_output([const.TERRAFORM_BIN, 'output'])

        lustre_node_info = result.splitlines()
        client01_ip = None
        client01_hostname = None
        client02_ip = None
        client02_hostname = None
        mds01_ip = None
        mds01_hostname = None
        mds02_ip = None
        mds02_hostname = None
        ost01_ip = None
        ost01_hostname = None

        for info in lustre_node_info:
            node_info_str = info.decode('utf-8')
            node_info = node_info_str.split(" = ")
            if node_info[0] == const.TERRAFORM_CLIENT01_IP:
                client01_ip = node_info[1]
                self.node_ip_list.append(client01_ip)
            elif node_info[0] == const.TERRAFORM_CLIENT02_IP:
                client02_ip = node_info[1]
                self.node_ip_list.append(client02_ip)
            elif node_info[0] == const.TERRAFORM_MDS01_IP:
                mds01_ip = node_info[1]
                self.node_ip_list.append(mds01_ip)
            elif node_info[0] == const.TERRAFORM_MDS02_IP:
                mds02_ip = node_info[1]
                self.node_ip_list.append(mds02_ip)
            elif node_info[0] == const.TERRAFORM_OST01_IP:
                ost01_ip = node_info[1]
                self.node_ip_list.append(ost01_ip)
            elif node_info[0] == const.TERRAFORM_CLIENT01_HOSTNAME:
                client01_hostname = node_info[1]
            elif node_info[0] == const.TERRAFORM_CLIENT02_HOSTNAME:
                client02_hostname = node_info[1]
            elif node_info[0] == const.TERRAFORM_MDS01_HOSTNAME:
                mds01_hostname = node_info[1]
            elif node_info[0] == const.TERRAFORM_MDS02_HOSTNAME:
                mds02_hostname = node_info[1]
            elif node_info[0] == const.TERRAFORM_OST01_HOSTNAME:
                ost01_hostname = node_info[1]
            else:
                self._error("The node info is not correct.")

        with open(const.NODE_INFO, 'w+') as node_conf:
            node_conf.write(client01_hostname + ' ' + client01_ip + ' ' + const.CLIENT)
            node_conf.write(client02_hostname + ' ' + client02_ip + ' ' + const.CLIENT)
            node_conf.write(mds01_hostname + ' ' + mds01_ip + ' ' + const.MDS)
            node_conf.write(mds02_hostname + ' ' + mds02_ip + ' ' + const.MDS)
            node_conf.write(ost01_hostname + ' ' + ost01_ip + ' ' + const.OST)

    def node_check(self):
        # Check the node is alive and the cloud-init is finished.
        if len(self.node_ip_list) == 5:
            ssh_clients = []
            ssh_check_cmd = "ls -l " + const.CLOUD_INIT_FINISH
            while True:
                for ip in self.node_ip_list:
                    ssh_client = self.ssh_connection(ip)
                    if ssh_client is not None:
                        if ssh_client not in ssh_clients:
                            ssh_clients.append(ssh_client)

                if len(ssh_clients) == 5:
                    self._debug("All the clients is ready")
                    break
                else:
                    time.sleep(5)
            t1 = datetime.now()
            node_status = []
            while (datetime.now() - t1).seconds <= const.CLOUD_INIT_FINISH:
                for client in ssh_clients:
                    if client in node_status:
                        continue
                    else:
                        if self.ssh_exec(client, ssh_check_cmd):
                            node_status.append(client)
                        else:
                            self._error("The cloud-init process is not finished")
                ready_node = len(node_status)
                self._debug("Ready nodes: " + str(ready_node))

                time.sleep(10)

            if len(node_status) == 5:
                return True
            else:
                self._error("The cloud-init processes of nodes are "
                            "not totally ready, only ready: " + str(len(node_status)))
                return False

        else:
            self._error("Cluster node count is not right")
            return False

    def provision(self):
        self.prepare_tf_conf()
        if self.terraform_apply():
            self.write_to_file()
        else:
            self._error("Terraform apply process failed")
            return False

        # check the file is there
        if exists(const.NODE_INFO):
            if self.node_check():
                return True
        else:
            self._error("The config file does not exist: " + const.NODE_INFO)
            return False


def main():
    lustre_cluster_provisioner = Provisioner()
    lustre_cluster_provisioner.provision()

if __name__ == "__main__":
    main()
