UPDATE membership_types
SET
  summary = CASE
    WHEN membership_name = 'LYMPH CARE & RECOVERY' THEN
      E'Gentle, therapeutic support for the lymphatic system \u2014 designed to reduce swelling, improve circulation, and help your body recover naturally.'
    WHEN membership_name = 'DETOX & WELLBEING FLOW' THEN
      E'A gentle wellness rhythm to support energy, digestion, and balance \u2014 for bloating, fatigue, stress, and hormonal imbalance.'
    WHEN membership_name = 'WEIGHT LOSS & METABOLIC RESET' THEN
      E'Structured metabolic support without high-impact exercise \u2014 for fat loss, hormonal weight, and slow metabolism.'
    WHEN membership_name = 'BODY SCULPT & CELLULITE TRANSFORMATION' THEN
      E'Visible sculpting with mandatory lymphatic drainage support \u2014 for cellulite, localised fat, and body shaping.'
    WHEN membership_name = 'TONE UP MEMBERSHIP' THEN
      E'Firm, sculpt, strengthen, and define \u2014 for women who want to feel firmer, tighter, and more confident without the gym.'
    ELSE summary
  END,

  benefits = CASE
    WHEN membership_name = 'LYMPH CARE & RECOVERY' THEN
      E'\u2022 Reduced swelling and fluid retention\n\u2022 Less pain and heaviness\n\u2022 Improved mobility and comfort\n\u2022 Faster recovery\n\u2022 Better quality of life\n\u2022 Feeling safe, supported, and confident in your journey'
    WHEN membership_name = 'DETOX & WELLBEING FLOW' AND membership_level = 'RESET' THEN
      E'\u2022 Reduced bloating and digestive discomfort\n\u2022 Improved energy levels\n\u2022 Better stress resilience\n\u2022 More balanced hormonal wellbeing\n\u2022 A sense of lightness and renewed vitality'
    WHEN membership_name = 'DETOX & WELLBEING FLOW' AND membership_level = 'FLOW' THEN
      E'\u2022 Reduced bloating and digestive discomfort\n\u2022 Improved and sustained energy levels\n\u2022 Better stress resilience and hormonal balance\n\u2022 More consistent week-to-week wellbeing\n\u2022 A sense of lightness and renewed vitality'
    WHEN membership_name = 'DETOX & WELLBEING FLOW' AND membership_level = 'WELLBEING LIFESTYLE' THEN
      E'\u2022 Significantly reduced bloating and digestive issues\n\u2022 Stronger, more sustained energy levels\n\u2022 Improved hormonal balance and stress resilience\n\u2022 Compounding results week after week\n\u2022 A deep sense of lightness and vitality'
    WHEN membership_name = 'WEIGHT LOSS & METABOLIC RESET' AND membership_level = 'KICKSTART' THEN
      E'\u2022 Visible body-composition improvements\n\u2022 Boosted metabolism without high-impact exercise\n\u2022 Reduced stubborn fat areas\n\u2022 Improved energy and motivation\n\u2022 Sustainable weight-loss momentum'
    WHEN membership_name = 'WEIGHT LOSS & METABOLIC RESET' AND membership_level = 'TRANSFORM' THEN
      E'\u2022 Stronger weekly fat-loss progress\n\u2022 Visible body-shape change\n\u2022 Boosted metabolism without high-impact exercise\n\u2022 Improved confidence and accountability\n\u2022 Sustainable, long-term results'
    WHEN membership_name = 'WEIGHT LOSS & METABOLIC RESET' AND membership_level = 'ACCELERATE' THEN
      E'\u2022 Fastest possible fat-loss results\n\u2022 Maximum metabolic stimulation\n\u2022 Dramatic body-shape transformation\n\u2022 Premium structure and accountability\n\u2022 Sustained long-term results'
    WHEN membership_name = 'BODY SCULPT & CELLULITE TRANSFORMATION' AND membership_level = 'SHAPE' THEN
      E'\u2022 Visible cellulite reduction\n\u2022 Improved body contour and shape\n\u2022 Smoother, firmer skin texture\n\u2022 Better circulation and lymphatic function\n\u2022 Increased body confidence'
    WHEN membership_name = 'BODY SCULPT & CELLULITE TRANSFORMATION' AND membership_level = 'SCULPT' THEN
      E'\u2022 Faster cellulite reduction\n\u2022 More defined body contour\n\u2022 Smoother, firmer skin texture\n\u2022 Accelerated sculpting progress week over week\n\u2022 Increased body confidence'
    WHEN membership_name = 'BODY SCULPT & CELLULITE TRANSFORMATION' AND membership_level = 'ELITE SCULPT' THEN
      E'\u2022 Maximum-speed cellulite reduction\n\u2022 Dramatic body contour transformation\n\u2022 Premium skin smoothness and firmness\n\u2022 The fastest sculpting results we offer\n\u2022 Ultimate body confidence'
    WHEN membership_name = 'TONE UP MEMBERSHIP' AND membership_level = 'TONE LITE' THEN
      E'\u2022 Improved muscle tone and firmness\n\u2022 Better posture and body confidence\n\u2022 Tighter, more defined body shape\n\u2022 Visible results within the 12-week program\n\u2022 A stronger, more confident you'
    WHEN membership_name = 'TONE UP MEMBERSHIP' AND membership_level = 'TONE SCULPT' THEN
      E'\u2022 Noticeable firmness and muscle definition\n\u2022 Visible body shaping within the 12-week program\n\u2022 Improved posture and confidence\n\u2022 Stronger, more toned physique\n\u2022 Real, lasting body confidence'
    WHEN membership_name = 'TONE UP MEMBERSHIP' AND membership_level = 'TONE PRO' THEN
      E'\u2022 Maximum muscle definition and firmness\n\u2022 The fastest toning results we offer\n\u2022 Dramatic transformation within 12 weeks\n\u2022 Premium structure and accountability\n\u2022 Ultimate body confidence'
    ELSE benefits
  END,

  updated_at = NOW()
WHERE tenant_id = (SELECT id FROM tenants WHERE slug = 'cnektp')
  AND deleted_at IS NULL;

-- Also strip the "Expected Benefits:" section from description since it now lives in its own field
-- We won't do this automatically since descriptions were carefully crafted - just populate the new fields.

SELECT membership_name, membership_level,
  length(summary) AS sum_len,
  length(description) AS desc_len,
  length(inclusions) AS inc_len,
  length(benefits) AS ben_len
FROM membership_types
WHERE tenant_id = (SELECT id FROM tenants WHERE slug = 'cnektp')
  AND deleted_at IS NULL
ORDER BY membership_name, membership_level;
