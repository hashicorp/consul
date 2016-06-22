
VAGRANTFILE_API_VERSION = "2"

Vagrant.configure(VAGRANTFILE_API_VERSION) do |config|

  config.vm.box = "hashicorp/precise64"

  config.vm.define :n1 do |n1|
    n1.vm.hostname = "n1"
    n1.vm.network "private_network", ip: "172.20.20.10"
    n1.vm.provision "shell" do |s|
      s.path = "provision.sh"
      s.args = ["n1", "N"]
    end
  end

  config.vm.define :n2 do |n2|
    n2.vm.hostname = "n2"
    n2.vm.network "private_network", ip: "172.20.20.11"
    n2.vm.network "forwarded_port", guest: 8500, host: 8501
    n2.vm.provision "shell" do |s|
      s.path = "provision.sh"
      s.args = ["n2", "Y"]
    end
  end

end
