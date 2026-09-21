Vagrant.configure("2") do |config|
  # Use an ARM64-compatible Ubuntu image for Apple Silicon
  config.vm.box = "spox/ubuntu-arm"
  config.vm.box_version = "1.0.0"

  # Solve Blocker #2: Explicit Port Forwarding
  config.vm.network "forwarded_port", guest: 8080, host: 8080

  # Provisioning using VMware
  config.vm.provider "vmware_desktop" do |v|
    v.vmx["memsize"] = "2048"
    v.vmx["numvcpus"] = "2"
  end

  # Provisioning: Install Postgres and set up the legacy DB
  config.vm.provision "shell", inline: <<-SHELL
    echo "Updating packages and installing PostgreSQL..."
    apt-get update
    apt-get install -y postgresql postgresql-contrib

    echo "Setting up the TaskMinds database and user..."
    sudo -u postgres psql -c "CREATE USER legacy_user WITH PASSWORD 'supersecret';"
    sudo -u postgres psql -c "CREATE DATABASE inventory_db OWNER legacy_user;"
    
    echo "Legacy server provisioned successfully!"
  SHELL
end