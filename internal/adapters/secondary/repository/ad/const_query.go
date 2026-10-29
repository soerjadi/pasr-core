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
