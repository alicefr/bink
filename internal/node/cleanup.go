package node

import (
	"github.com/sirupsen/logrus"
)

// Cleanup performs cleanup operations for the node
// This includes stopping the virtiofsd process if it's running
func (n *Node) Cleanup() error {
	if n.virtiofsdMgr != nil {
		logrus.Debugf("Cleaning up virtiofsd for node %s", n.Name)
		if err := n.virtiofsdMgr.Stop(); err != nil {
			logrus.Warnf("Failed to stop virtiofsd for node %s: %v", n.Name, err)
			return err
		}
	}
	return nil
}
