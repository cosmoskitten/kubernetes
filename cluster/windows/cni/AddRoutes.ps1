$endpointName = "cbr0"
$vnicName = "vEthernet ($endpointName)"

function
Add-RouteToPodCIDR()
{
    '''
    $route = get-netroute -InterfaceAlias "$vnicName" -DestinationPrefix $clusterCIDR -erroraction Ignore
    if (!$route) 
    {
        New-Netroute -DestinationPrefix $clusterCIDR -InterfaceAlias "$vnicName" -NextHop 0.0.0.0 -Verbose
    }

    '''
    $podCIDRs=c:\k\kubectl.exe  --kubeconfig=c:\k\config get nodes -o=custom-columns=Name:.status.nodeInfo.operatingSystem,PODCidr:.spec.podCIDR --no-headers
    Write-Host "Add-RouteToPodCIDR - available nodes $podCIDRs"
    foreach ($podcidr in $podCIDRs)
    {
        $tmp = $podcidr.Split(" ")
        $os = $tmp | select -First 1
        $cidr = $tmp | select -Last 1
        $cidrGw =  $cidr.substring(0,$cidr.lastIndexOf(".")) + ".1"

        
        if ($os -eq "windows") {
            $cidrGw = $cidr.substring(0,$cidr.lastIndexOf(".")) + ".2"
        }

        Write-Host "Adding route for Remote Pod CIDR $cidr, GW $cidrGw, for node type $os"

        $route = get-netroute -InterfaceAlias "$vnicName" -DestinationPrefix $cidr -erroraction Ignore
        if (!$route) {
            new-netroute -InterfaceAlias "$vnicName" -DestinationPrefix $cidr -NextHop  $cidrGw -Verbose
        }
    }
}

Add-RouteToPodCIDR