-- 1. Remove "Expected Benefits:" section from descriptions since it's now in the separate benefits field.
-- 2. Add session consistency/rollover policy to LYMPH CARE descriptions.

-- LYMPH CARE & RECOVERY - SUPPORT: strip benefits section, add rollover policy
UPDATE membership_types
SET description = E'For Lymphedema \u2022 Lipedema \u2022 Post-Surgery \u2022 Chronic Swelling \u2022 Limited Mobility

This membership is designed to support your lymphatic system safely and gently. It focuses on reducing swelling, improving circulation, and helping your body recover naturally.

The Support level gives you two dedicated sessions per week \u2014 the ideal starting point for building consistency without overwhelming your schedule. Many clients notice reduced tightness and improved comfort within the first few weeks.

Each 60\u201390 minute session combines one core lymphatic treatment with one support treatment, using gentle, targeted techniques to encourage natural fluid movement and ease discomfort.

Treatments Used:
\u2022 Roll Shaper \u2013 supports natural lymph flow and circulation
\u2022 Vacu Shaper (low pressure only) \u2013 gentle lymphatic stimulation
\u2022 Lymphatic Compression Massage \u2013 supports fluid movement and drainage
\u2022 Collagen Red Light Therapy \u2013 helps tissue recovery and healing

Because lymphatic conditions require consistent care, sessions are designed to be attended regularly. Biologically, it takes 3\u20134 weeks to wake up a sluggish lymphatic system \u2014 so consistency is where the real results happen.

Important: Session Consistency
Sessions cannot be rolled over week to week. Skipping sessions can lead to increased swelling, fluid returning, and reduced treatment results. One rollover per month may be granted for medical reasons only.

This membership supports your lymphatic system gently and consistently \u2014 because your body needs care, not force.',
updated_at = NOW()
WHERE membership_name = 'LYMPH CARE & RECOVERY' AND membership_level = 'SUPPORT'
  AND tenant_id = (SELECT id FROM tenants WHERE slug = 'cnektp') AND deleted_at IS NULL;

-- LYMPH CARE & RECOVERY - THERAPEUTIC
UPDATE membership_types
SET description = E'For Lymphedema \u2022 Lipedema \u2022 Post-Surgery \u2022 Chronic Swelling \u2022 Limited Mobility

This is our most chosen lymphatic membership \u2014 and for good reason.

Three sessions per week gives your body the frequency it needs to build real momentum. Clients at this level consistently report faster relief, better mobility, and more noticeable week-to-week progress compared to two sessions alone.

Each 60\u201390 minute session combines one core lymphatic treatment with one support treatment, using gentle, targeted techniques designed to reduce swelling, improve circulation, and help your body recover naturally.

Treatments Used:
\u2022 Roll Shaper \u2013 supports natural lymph flow and circulation
\u2022 Vacu Shaper (low pressure only) \u2013 gentle lymphatic stimulation
\u2022 Lymphatic Compression Massage \u2013 supports fluid movement and drainage
\u2022 Collagen Red Light Therapy \u2013 helps tissue recovery and healing

Example Weekly Schedule:
Monday \u2013 Roll Shaper + Vacu Shaper
Wednesday \u2013 Lymphatic Compression + Collagen Red Light
Friday \u2013 Roll Shaper + Vacu Shaper

This is the sweet spot between consistency and sustainability \u2014 strong enough frequency for meaningful results, manageable enough to maintain long-term.

Important: Session Consistency
Sessions cannot be rolled over week to week. Skipping sessions can lead to increased swelling, fluid returning, and reduced treatment results. One rollover per month may be granted for medical reasons only.

This membership supports your lymphatic system gently and consistently \u2014 because your body needs care, not force.',
updated_at = NOW()
WHERE membership_name = 'LYMPH CARE & RECOVERY' AND membership_level = 'THERAPEUTIC'
  AND tenant_id = (SELECT id FROM tenants WHERE slug = 'cnektp') AND deleted_at IS NULL;

-- LYMPH CARE & RECOVERY - INTENSIVE
UPDATE membership_types
SET description = E'For Lymphedema \u2022 Lipedema \u2022 Post-Surgery \u2022 Chronic Swelling \u2022 Limited Mobility

When your body needs more, this is the membership that delivers. Four sessions per week provides the highest level of structured lymphatic support we offer.

It\u2019s built for clients managing complex swelling, recovering from surgery, or going through flare periods where skipping days sets you back. This level gives your body close, consistent care for the fastest path to reduced swelling and stronger daily function.

Each 60\u201390 minute session combines one core lymphatic treatment with one support treatment, using gentle, targeted techniques designed to reduce swelling, improve circulation, and help your body recover naturally.

Treatments Used:
\u2022 Roll Shaper \u2013 supports natural lymph flow and circulation
\u2022 Vacu Shaper (low pressure only) \u2013 gentle lymphatic stimulation
\u2022 Lymphatic Compression Massage \u2013 supports fluid movement and drainage
\u2022 Collagen Red Light Therapy \u2013 helps tissue recovery and healing

At this frequency, sessions are spread across the week to maintain steady lymphatic stimulation. Consistency is essential \u2014 biologically, it takes 3\u20134 weeks to wake up a sluggish lymphatic system, and maintaining frequency is what keeps progress moving forward.

Important: Session Consistency
Sessions cannot be rolled over week to week. Skipping sessions can lead to increased swelling, fluid returning, and reduced treatment results. One rollover per month may be granted for medical reasons only.

This membership supports your lymphatic system gently and consistently \u2014 because your body needs care, not force.',
updated_at = NOW()
WHERE membership_name = 'LYMPH CARE & RECOVERY' AND membership_level = 'INTENSIVE'
  AND tenant_id = (SELECT id FROM tenants WHERE slug = 'cnektp') AND deleted_at IS NULL;

-- Now strip "Expected Benefits:" section from ALL other descriptions since benefits are in their own field now
-- DETOX - RESET
UPDATE membership_types SET description = regexp_replace(description, E'\n\nExpected Benefits:.*$', '', 's'), updated_at = NOW()
WHERE membership_name = 'DETOX & WELLBEING FLOW' AND membership_level = 'RESET'
  AND tenant_id = (SELECT id FROM tenants WHERE slug = 'cnektp') AND deleted_at IS NULL;

UPDATE membership_types SET description = regexp_replace(description, E'\n\nExpected Benefits:.*$', '', 's'), updated_at = NOW()
WHERE membership_name = 'DETOX & WELLBEING FLOW' AND membership_level = 'FLOW'
  AND tenant_id = (SELECT id FROM tenants WHERE slug = 'cnektp') AND deleted_at IS NULL;

UPDATE membership_types SET description = regexp_replace(description, E'\n\nExpected Benefits:.*$', '', 's'), updated_at = NOW()
WHERE membership_name = 'DETOX & WELLBEING FLOW' AND membership_level = 'WELLBEING LIFESTYLE'
  AND tenant_id = (SELECT id FROM tenants WHERE slug = 'cnektp') AND deleted_at IS NULL;

UPDATE membership_types SET description = regexp_replace(description, E'\n\nExpected Benefits:.*$', '', 's'), updated_at = NOW()
WHERE membership_name = 'WEIGHT LOSS & METABOLIC RESET'
  AND tenant_id = (SELECT id FROM tenants WHERE slug = 'cnektp') AND deleted_at IS NULL;

UPDATE membership_types SET description = regexp_replace(description, E'\n\nExpected Benefits:.*$', '', 's'), updated_at = NOW()
WHERE membership_name = 'BODY SCULPT & CELLULITE TRANSFORMATION'
  AND tenant_id = (SELECT id FROM tenants WHERE slug = 'cnektp') AND deleted_at IS NULL;

UPDATE membership_types SET description = regexp_replace(description, E'\n\nExpected Benefits:.*$', '', 's'), updated_at = NOW()
WHERE membership_name = 'TONE UP MEMBERSHIP'
  AND tenant_id = (SELECT id FROM tenants WHERE slug = 'cnektp') AND deleted_at IS NULL;

-- Verify
SELECT membership_name, membership_level,
  length(summary) AS sum_len,
  length(description) AS desc_len,
  length(inclusions) AS inc_len,
  length(benefits) AS ben_len,
  CASE WHEN description LIKE '%Expected Benefits%' THEN 'STILL HAS BENEFITS IN DESC' ELSE 'clean' END AS desc_check,
  CASE WHEN description LIKE '%rollover%' OR membership_name != 'LYMPH CARE & RECOVERY' THEN 'ok' ELSE 'MISSING ROLLOVER' END AS rollover_check
FROM membership_types
WHERE tenant_id = (SELECT id FROM tenants WHERE slug = 'cnektp')
  AND deleted_at IS NULL
ORDER BY membership_name, membership_level;
