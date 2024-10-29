## Usage

[Helm](https://helm.sh) must be installed to use the charts.  Please refer to
Helm's [documentation](https://helm.sh/docs) to get started.

Once Helm has been set up correctly, add the repo as follows:

    helm repo add backup-ns https://code.allaboutapps.at/backup-ns

If you had already added this repo earlier, run `helm repo update` to retrieve
the latest versions of the packages.  You can then run `helm search repo
backup-ns` to see the charts.

To install the backup-ns chart:

    helm install my-backup-ns backup-ns/backup-ns

To uninstall the chart:

    helm delete my-backup-ns