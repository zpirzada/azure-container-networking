#This script  is to invoke cni for windows containers. It has an option to configure dns related info via cni.
Param(
	[parameter(Mandatory=$true)]
	[string] $containerName,

	[parameter(Mandatory=$true)]
	[string] $namespace,

	[parameter(Mandatory=$true)]
	[string] $contid,

	[parameter (Mandatory=$true)]
	[string] $command,

	[parameter (Mandatory=$false)]
	[string] $dns,

	[parameter (Mandatory=$false)]
	[string] $dnssuffix,

	[parameter (Mandatory=$false)]
	[string] $netns,

	[parameter (Mandatory=$false)]
	[string] $cnidir,

	[parameter (Mandatory=$false)]
	[string] $confPath
)


$env:CNI_CONTAINERID=$contid
$env:CNI_COMMAND=$command


$k8sargs='IgnoreUnknown=1;K8S_POD_NAMESPACE={0};K8S_POD_NAME={1};K8S_POD_INFRA_CONTAINER_ID={2}' -f $namespace, $containerName, $contid
$env:CNI_ARGS=$k8sargs
$env:CNI_IFNAME='eth0'

if ($netns -eq "") {
	$netns='none'
}
$env:CNI_NETNS=$netns

if ($cnidir -eq "") {
	$cnidir='C:\k\azurecni\bin'
}
$env:CNI_PATH=$cnidir
$env:PATH="$env:CNI_PATH;"+$env:PATH

if ($confPath -eq "") {
	$confPath='C:\k\azurecni\netconf\10-azure.conflist'
}

<#
usage:
.\invoke-cni.ps1 <container_name> <namespace> <container_id> [ADD/DEL] <dns_array> <dns_suffix>
<dns_array> - values should be quoted and comma separated
<dns_suffix> - values should be quoted and comma separated
eg: .\cni.ps1 container1 default 01fb3472a90a2ee282b8f15665bd38dc76100922fc7c9c5dd689f578231c9b97 ADD '"1.2.3.4"' '"asd.net"'
#>

#read conflist and extract plugin component

$content = Get-Content -Raw -Path $confPath

$jobj = ConvertFrom-Json $content
$plugin=$jobj.plugins[0]

# add name and version in plugin section
$plugin|add-member -Name "name" -Value $jobj.name -MemberType Noteproperty
$plugin|add-member -Name "cniVersion" -Value $jobj.cniVersion -MemberType Noteproperty

#remove array datatype as it adds name and value by default
$arrayDataType = get-TypeData  System.Array
Remove-TypeData  System.Array

#add dnsserver and dnssuffix(searches) as runtimeconfig parameters
if ($dns -ne "" -Or $dnssuffix -ne "") {
	$dnsjson = "[" + $dns + "]"
	$serverarray = convertfrom-json $dnsjson
	$configobj = New-Object -TypeName PSObject
	# add servers array to config object
	$configobj|add-member -Name "servers" -Value $serverarray -MemberType Noteproperty
	$dnssuffixjson = "[" + $dnssuffix + "]"
	$searcharray = convertfrom-json $dnssuffixjson
	# add searches array to config object
	$configobj|add-member -Name "searches" -Value $searcharray -MemberType Noteproperty

	# add config object child of dns object
	$dnsobj = New-Object -TypeName PSObject
	$dnsobj|add-member -Name "dns" -Value $configobj -MemberType Noteproperty

	# add dns object as child to plugin object
	$plugin|add-member -Name "runtimeConfig" -Value $dnsobj -MemberType Noteproperty
}

$jsonconfig=ConvertTo-Json $plugin -Depth 6
echo $jsonconfig
$res=(echo $jsonconfig | azure-vnet)
echo $res
#restore the array datatype
Update-TypeData -TypeData $arrayDataType