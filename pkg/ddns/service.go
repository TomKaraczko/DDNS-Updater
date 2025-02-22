package ddns

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/plaenkler/ddns-updater/pkg/config"
	"github.com/plaenkler/ddns-updater/pkg/database"
	"github.com/plaenkler/ddns-updater/pkg/database/model"
	log "github.com/plaenkler/ddns-updater/pkg/logging"
	"gorm.io/gorm"
)

var (
	mu   sync.Mutex
	stop chan bool
)

func StartService() {
	mu.Lock()
	defer mu.Unlock()
	stop = make(chan bool)
	interval := time.Second * time.Duration(config.GetConfig().Interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			updateInterval(interval, ticker)
			address, err := GetPublicIP()
			if err != nil {
				log.Errorf("[ddns-StartService-1] failed to get public IP address - error: %v", err)
				continue
			}
			newAddress := model.IPAddress{
				Address: address,
			}
			db := database.GetDatabase()
			if db == nil {
				log.Errorf("[ddns-StartService-2] failed to get database connection")
				continue
			}
			err = db.FirstOrCreate(&newAddress, newAddress).Error
			if err != nil {
				log.Errorf("[ddns-StartService-3] failed to save new IP address - error: %v", err)
				continue
			}
			jobs := getSyncJobs(db, newAddress.ID)
			if len(jobs) == 0 {
				log.Infof("[ddns-StartService-4] no dynamic DNS record needs to be updated")
				continue
			}
			updateDDNSEntries(db, jobs, newAddress)
		case <-stop:
			return
		}
	}
}

func updateInterval(interval time.Duration, ticker *time.Ticker) {
	newInterval := time.Second * time.Duration(config.GetConfig().Interval)
	if interval != newInterval && newInterval > 0 {
		ticker.Reset(newInterval)
		log.Infof("[ddns-updateInterval-1] changed interval from %v to %v", interval, newInterval)
	}
}

func getSyncJobs(db *gorm.DB, addressID uint) []model.SyncJob {
	var jobs []model.SyncJob
	err := db.Where("NOT ip_address_id = ? OR ip_address_id IS NULL", addressID).Find(&jobs).Error
	if err != nil {
		log.Errorf("[ddns-getSyncJobs-1] failed to get DDNS update jobs - error: %v", err)
		return nil
	}
	return jobs
}

func updateDDNSEntries(db *gorm.DB, jobs []model.SyncJob, a model.IPAddress) {
	for _, job := range jobs {
		updater, ok := updaters[job.Provider]
		if !ok {
			log.Errorf("[ddns-updateDDNSEntries-1] no updater found for job %v", job.ID)
			continue
		}
		request := updater.Request
		err := json.Unmarshal([]byte(job.Params), request)
		if err != nil {
			log.Errorf("[ddns-updateDDNSEntries-2] failed to unmarshal job params for job %v - error: %s", job.ID, err)
			continue
		}
		err = updater.Updater(request, a.Address)
		if err != nil {
			log.Errorf("[ddns-updateDDNSEntries-3] failed to update DDNS entry for job %v - error: %s", job.ID, err)
			continue
		}
		err = db.Model(&job).Update("ip_address_id", a.ID).Error
		if err != nil {
			log.Errorf("[ddns-updateDDNSEntries-4] failed to update IP address for job %v - error: %s", job.ID, err)
		}
		log.Infof("[ddns-updateDDNSEntries-5] updated DDNS entry for ID: %v Provider: %s Params: %+v", job.ID, job.Provider, job.Params)
	}
}

func StopService() {
	stop <- true
}
