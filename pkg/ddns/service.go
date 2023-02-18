package ddns

import (
	"log"
	"sync"
	"time"

	"github.com/plaenkler/ddns/pkg/config"
	"github.com/plaenkler/ddns/pkg/database"
	"github.com/plaenkler/ddns/pkg/model"
	"github.com/plaenkler/ddns/pkg/util"
)

var start sync.Once

func Start() {
	start.Do(func() {
		ticker := time.NewTicker(time.Second * time.Duration(config.GetConfig().Interval))
		defer ticker.Stop()
		for range ticker.C {
			address, err := util.GetPublicIP()
			if err != nil {
				log.Printf("[service-start-1] failed to get public IP address - error: %v", err)
				continue
			}
			newAddress := model.IPAddress{
				Address: address,
			}
			err = database.GetManager().DB.FirstOrCreate(&newAddress).Error
			if err != nil {
				log.Printf("[service-start-2] failed to save new IP address - error: %v", err)
				continue
			}
			jobs := []model.SyncJob{}
			err = database.GetManager().DB.Where("NOT ip_address_id = ? or ip_address_id IS NULL", newAddress.ID).Find(&jobs).Error
			if err != nil {
				log.Printf("[service-start-3] failed to get DDNS update jobs - error: %v", err)
				continue
			}
			for _, job := range jobs {
				resolver, ok := updaters[job.Provider]
				if !ok {
					log.Printf("[service-start-4] no updater found for job %v", job.ID)
					continue
				}
				err = resolver(job, address)
				if err != nil {
					log.Printf("[service-start-5] failed to update DDNS entry for %q: %v", job.Domain, err)
					continue
				}
				err = database.GetManager().DB.Model(&job).Update("ip_address_id", newAddress.ID).Error
				if err != nil {
					log.Printf("[service-start-6] failed to update IP address for job %v", job.ID)
				}
			}
		}
	})
}
