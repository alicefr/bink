package node

import (
	"crypto/md5"
	"fmt"

	"github.com/bootc-dev/bink/internal/config"
)

func CalculateClusterIP(nodeName string) string {
	hash := md5.Sum([]byte(nodeName))
	suffix := int(hash[0])%config.ClusterIPRangeSize + config.ClusterIPMinSuffix
	return fmt.Sprintf("%s.%d", config.ClusterIPPrefix, suffix)
}

func CalculateClusterMAC(nodeName string) string {
	hash := md5.Sum([]byte(nodeName))
	return fmt.Sprintf("%s:%02x:%02x:%02x", config.ClusterMACPrefix, hash[0], hash[1], hash[2])
}
