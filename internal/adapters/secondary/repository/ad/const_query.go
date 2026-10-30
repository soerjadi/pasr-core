package ad

const listActiveQuery = `
	SELECT id, placement, title, subtitle, image, target_url, label
	FROM ad_campaigns
	WHERE placement = $1
	  AND active
	  AND (starts_at IS NULL OR starts_at <= $2)
	  AND (ends_at IS NULL OR $2 < ends_at)
	ORDER BY priority DESC, id
`

const campaignColumns = `id, placement, advertiser, title, subtitle, image, target_url, label, priority, active, starts_at, ends_at`

const (
	listCampaignsQuery = `
		SELECT ` + campaignColumns + `
		FROM ad_campaigns
		WHERE ($1::bigint = 0 OR id < $1)
		ORDER BY id DESC
		LIMIT $2
	`
	lockCampaignQuery   = `SELECT ` + campaignColumns + ` FROM ad_campaigns WHERE id = $1 FOR UPDATE`
	createCampaignQuery = `
		INSERT INTO ad_campaigns (placement, advertiser, title, subtitle, image, target_url, label, priority, active, starts_at, ends_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING ` + campaignColumns
	updateCampaignQuery = `
		UPDATE ad_campaigns SET placement = $2, advertiser = $3, title = $4, subtitle = $5, image = $6,
			target_url = $7, label = $8, priority = $9, active = $10, starts_at = $11, ends_at = $12
		WHERE id = $1
		RETURNING ` + campaignColumns
	deleteCampaignQuery = `DELETE FROM ad_campaigns WHERE id = $1`
)
