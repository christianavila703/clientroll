UPDATE membership_types
SET
  description = CASE

    WHEN membership_name = 'LYMPH CARE & RECOVERY' AND membership_level = 'SUPPORT' THEN
E'If you live with swelling, heaviness, or fluid retention \u2014 and you want consistent, gentle care that actually helps \u2014 this is where you start.\n\nThe Support level gives you two dedicated sessions each week, designed to encourage natural fluid movement, ease discomfort, and help your body find a steadier rhythm. Many clients notice reduced tightness and improved comfort within the first few weeks.\n\nThis membership suits you if you want structured weekly support without overwhelming your schedule. It\u2019s our foundation level \u2014 steady, effective, and designed to build confidence in your body again.'

    WHEN membership_name = 'LYMPH CARE & RECOVERY' AND membership_level = 'THERAPEUTIC' THEN
E'This is our most chosen lymphatic plan \u2014 and for good reason.\n\nThree sessions per week gives your body the frequency it needs to build real momentum. Clients at this level consistently report faster relief, better mobility, and more noticeable week-to-week progress compared to two sessions alone.\n\nTherapeutic suits you if swelling, post-surgical recovery, or chronic fluid retention is affecting your quality of life and you want meaningful results without going to the highest intensity. It\u2019s the sweet spot between consistency and sustainability \u2014 which is why most of our lymphatic clients choose this level.'

    WHEN membership_name = 'LYMPH CARE & RECOVERY' AND membership_level = 'INTENSIVE' THEN
E'When your body needs more, this is the membership that delivers.\n\nFour sessions per week provides the highest level of structured lymphatic support we offer. It\u2019s built for clients managing complex swelling, recovering from surgery, or going through flare periods where skipping days sets you back.\n\nIntensive suits you if you need close, consistent care and want the fastest path to reduced swelling, improved comfort, and stronger daily function. This is serious support for serious needs \u2014 guided, safe, and built around your recovery.'


    WHEN membership_name = 'DETOX & WELLBEING FLOW' AND membership_level = 'RESET' THEN
E'Feeling bloated, sluggish, or just off? Reset is the simplest way to give your body a weekly tune-up.\n\nOne session per week is all it takes to stimulate drainage, ease that heavy-gut feeling, and bring a little more lightness back into your day. It\u2019s not about dramatic changes \u2014 it\u2019s about giving your body a consistent reset so the good days start to outnumber the bad ones.\n\nThis membership suits you if you want a manageable starting point \u2014 something that fits easily into your week while still making a noticeable difference to how you feel.'

    WHEN membership_name = 'DETOX & WELLBEING FLOW' AND membership_level = 'FLOW' THEN
E'Two sessions per week creates the rhythm most clients need to feel genuinely different \u2014 not just on session days, but all week long.\n\nFlow is our most popular detox level because it balances real results with real life. Clients at this level report better energy, lighter digestion, reduced stress load, and a sense of consistency that one session alone can\u2019t sustain.\n\nThis membership suits you if bloating, fatigue, or hormonal imbalance is dragging you down and you want a structured wellness rhythm that actually sticks.'

    WHEN membership_name = 'DETOX & WELLBEING FLOW' AND membership_level = 'WELLBEING LIFESTYLE' THEN
E'For clients who\u2019ve decided wellness isn\u2019t a phase \u2014 it\u2019s a lifestyle.\n\nThree sessions per week gives you deeper, more sustained support across energy, digestion, stress resilience, and hormonal balance. This level builds on the Flow rhythm and takes it further, creating the kind of consistency where results compound and setbacks become rare.\n\nThis membership suits you if you\u2019re committed to feeling your best long-term and want a structured, high-frequency wellness plan that becomes part of your weekly routine.'


    WHEN membership_name = 'WEIGHT LOSS & METABOLIC RESET' AND membership_level = 'KICKSTART' THEN
E'If traditional diets and gym plans haven\u2019t worked \u2014 or you\u2019re tired of the yo-yo cycle \u2014 Kickstart gives you a smarter entry point.\n\nTwo sessions per week provide structured metabolic stimulation designed to restart fat-loss without high-impact exercise. This is about creating early wins, building consistency, and showing your body what\u2019s possible when the approach is right.\n\nThis membership suits you if you want to lose weight sustainably, you prefer a gentler approach, and you\u2019re ready for something that actually fits your life.'

    WHEN membership_name = 'WEIGHT LOSS & METABOLIC RESET' AND membership_level = 'TRANSFORM' THEN
E'This is where real body-shape change happens \u2014 and it\u2019s why Transform is our most popular metabolic membership.\n\nThree sessions per week gives you the frequency needed for stronger weekly progress, visible fat-loss momentum, and the kind of accountability that keeps you moving forward even on tough weeks. Clients at this level consistently see faster results and feel more in control of their journey.\n\nThis membership suits you if you\u2019re serious about changing your body, you want structured support, and you\u2019re ready to commit to a plan that delivers.'

    WHEN membership_name = 'WEIGHT LOSS & METABOLIC RESET' AND membership_level = 'ACCELERATE' THEN
E'For clients who want maximum pace, maximum structure, and no excuses.\n\nFour sessions per week is our highest-frequency metabolic plan \u2014 built for people who are fully committed to transformation and want to see change as fast as their body allows. More sessions mean stronger metabolic adaptation, faster progress, and premium accountability.\n\nThis membership suits you if you\u2019ve got a goal, a timeline, and the willingness to show up. This is the fast lane.'


    WHEN membership_name = 'BODY SCULPT & CELLULITE TRANSFORMATION' AND membership_level = 'SHAPE' THEN
E'If cellulite, stubborn areas, or uneven texture is bothering you \u2014 and you want to see genuine change \u2014 Shape is the most effective starting point.\n\nTwo sessions per week combine targeted sculpting with essential lymphatic drainage to improve skin smoothness, body contour, and overall confidence. This isn\u2019t a quick fix \u2014 it\u2019s a structured plan that builds visible results over time.\n\nThis membership suits you if you want a proven sculpting rhythm that fits your week without overcommitting. It\u2019s our most popular level for a reason.'

    WHEN membership_name = 'BODY SCULPT & CELLULITE TRANSFORMATION' AND membership_level = 'SCULPT' THEN
E'Ready to step it up? Sculpt adds a third session to increase treatment stimulus and speed up visible changes in contour, cellulite reduction, and overall body shape.\n\nThis level suits clients who\u2019ve seen what two sessions can do and want more \u2014 or clients who are starting with a clear goal and want to get there faster. The extra frequency makes a measurable difference in how quickly results show.\n\nThis membership suits you if you\u2019re ready to push harder and want noticeable, consistent sculpting progress week after week.'

    WHEN membership_name = 'BODY SCULPT & CELLULITE TRANSFORMATION' AND membership_level = 'ELITE SCULPT' THEN
E'Our highest-tier sculpting membership for clients who want the best results in the shortest time.\n\nFour sessions per week delivers maximum sculpting frequency with full lymphatic support built in. Elite Sculpt is designed for advanced body-shaping goals \u2014 whether that\u2019s a specific event, a confidence milestone, or simply wanting the most from every week.\n\nThis membership suits you if you want premium structure, accelerated visual transformation, and the highest level of sculpting support we offer.'


    WHEN membership_name = 'TONE UP MEMBERSHIP' AND membership_level = 'TONE LITE' THEN
E'You don\u2019t want to lose weight \u2014 you just want to feel firmer, tighter, and more confident in your own skin. That\u2019s exactly what Tone Lite delivers.\n\nTwo sessions per week over a focused 12-week program is designed to improve muscle tone, body firmness, and posture confidence without a gym-heavy routine. It\u2019s gentle, structured, and built for women who want to look and feel better without extreme measures.\n\nThis membership suits you if you want to tone up at your own pace with consistent, guided support.'

    WHEN membership_name = 'TONE UP MEMBERSHIP' AND membership_level = 'TONE SCULPT' THEN
E'Our most popular toning membership \u2014 because three sessions per week is where visible shaping really kicks in.\n\nOver the full 12-week program, Tone Sculpt builds noticeable firmness, improved definition, and the kind of body confidence that comes from seeing real change in the mirror. Clients at this level consistently tell us they feel stronger, more defined, and more comfortable in their clothes.\n\nThis membership suits you if you want to tone and shape with real momentum and you\u2019re ready to commit to something that works.'

    WHEN membership_name = 'TONE UP MEMBERSHIP' AND membership_level = 'TONE PRO' THEN
E'For women who want maximum definition, faster sculpting, and premium structure \u2014 Tone Pro is the top tier.\n\nFour sessions per week across the 12-week program delivers the most intensive toning experience we offer. More frequency means faster adaptation, stronger definition, and a more dramatic finish-line transformation.\n\nThis membership suits you if you have a clear goal, you want to see change quickly, and you\u2019re ready for the highest level of toning support available.'

    ELSE description
  END,

  inclusions = CASE

    WHEN membership_name = 'LYMPH CARE & RECOVERY' AND membership_level = 'SUPPORT' THEN
E'\u2022 2 sessions per week (8+ sessions per month)\n\u2022 Each session includes one core lymphatic treatment plus one support treatment\n\u2022 Core treatments: Roll Shaper or Lymphatic Compression Massage\n\u2022 Support treatments: Vacu Shaper (low pressure) or Collagen Red Light Therapy\n\u2022 Session duration: 60\u201390 minutes\n\u2022 Progress tracking included\n\u2022 Macros & Meal Plan included\n\u2022 Minimum 6-month or 12-month commitment, then month-to-month'

    WHEN membership_name = 'LYMPH CARE & RECOVERY' AND membership_level = 'THERAPEUTIC' THEN
E'\u2022 3 sessions per week (12+ sessions per month)\n\u2022 Each session includes one core lymphatic treatment plus one support treatment\n\u2022 Core treatments: Roll Shaper or Lymphatic Compression Massage\n\u2022 Support treatments: Vacu Shaper (low pressure) or Collagen Red Light Therapy\n\u2022 Session duration: 60\u201390 minutes\n\u2022 Progress tracking included\n\u2022 Macros & Meal Plan included\n\u2022 Most Popular \u2014 the best balance of frequency and sustainability\n\u2022 Minimum 6-month or 12-month commitment, then month-to-month'

    WHEN membership_name = 'LYMPH CARE & RECOVERY' AND membership_level = 'INTENSIVE' THEN
E'\u2022 4 sessions per week (16+ sessions per month)\n\u2022 Each session includes one core lymphatic treatment plus one support treatment\n\u2022 Core treatments: Roll Shaper or Lymphatic Compression Massage\n\u2022 Support treatments: Vacu Shaper (low pressure) or Collagen Red Light Therapy\n\u2022 Session duration: 60\u201390 minutes\n\u2022 Progress tracking included\n\u2022 Macros & Meal Plan included\n\u2022 Highest frequency tier \u2014 built for complex needs or accelerated recovery\n\u2022 Minimum 6-month or 12-month commitment, then month-to-month'


    WHEN membership_name = 'DETOX & WELLBEING FLOW' AND membership_level = 'RESET' THEN
E'\u2022 1 session per week (4+ sessions per month)\n\u2022 Structured treatment session to support drainage, energy, and digestive comfort\n\u2022 Gentle entry point \u2014 ideal for clients new to wellness support\n\u2022 Progress tracking included\n\u2022 Minimum 6-month or 12-month commitment, then month-to-month'

    WHEN membership_name = 'DETOX & WELLBEING FLOW' AND membership_level = 'FLOW' THEN
E'\u2022 2 sessions per week (8+ sessions per month)\n\u2022 Structured treatments to support energy, digestion, hormonal balance, and stress\n\u2022 Most Popular \u2014 the best blend of results and affordability\n\u2022 Progress tracking included\n\u2022 Minimum 6-month or 12-month commitment, then month-to-month'

    WHEN membership_name = 'DETOX & WELLBEING FLOW' AND membership_level = 'WELLBEING LIFESTYLE' THEN
E'\u2022 3 sessions per week (12+ sessions per month)\n\u2022 High-frequency structured wellness sessions for sustained results\n\u2022 Deeper support across energy, resilience, digestion, and balance\n\u2022 Progress tracking included\n\u2022 Minimum 6-month or 12-month commitment, then month-to-month'


    WHEN membership_name = 'WEIGHT LOSS & METABOLIC RESET' AND membership_level = 'KICKSTART' THEN
E'\u2022 2 sessions per week (8+ sessions per month)\n\u2022 Structured metabolic stimulation treatments \u2014 no high-impact exercise required\n\u2022 Designed for sustainable fat-loss and early body-composition changes\n\u2022 Progress tracking included\n\u2022 Macros & Meal Plan included\n\u2022 Minimum 6-month or 12-month commitment, then month-to-month'

    WHEN membership_name = 'WEIGHT LOSS & METABOLIC RESET' AND membership_level = 'TRANSFORM' THEN
E'\u2022 3 sessions per week (12+ sessions per month)\n\u2022 Structured metabolic stimulation for stronger weekly fat-loss progress\n\u2022 Most Popular \u2014 best frequency for visible body-shape change\n\u2022 Progress tracking included\n\u2022 Macros & Meal Plan included\n\u2022 Minimum 6-month or 12-month commitment, then month-to-month'

    WHEN membership_name = 'WEIGHT LOSS & METABOLIC RESET' AND membership_level = 'ACCELERATE' THEN
E'\u2022 4 sessions per week (16+ sessions per month)\n\u2022 Maximum-frequency metabolic plan for fastest results\n\u2022 Built for committed clients with clear goals and timelines\n\u2022 Progress tracking included\n\u2022 Macros & Meal Plan included\n\u2022 Minimum 6-month or 12-month commitment, then month-to-month'


    WHEN membership_name = 'BODY SCULPT & CELLULITE TRANSFORMATION' AND membership_level = 'SHAPE' THEN
E'\u2022 2 sessions per week (8+ sessions per month)\n\u2022 Targeted sculpting treatments with mandatory lymphatic drainage support\n\u2022 Focused on skin smoothness, contour improvement, and cellulite reduction\n\u2022 Most Popular \u2014 the most effective sculpting starting point\n\u2022 Progress tracking included\n\u2022 Minimum 6-month or 12-month commitment, then month-to-month'

    WHEN membership_name = 'BODY SCULPT & CELLULITE TRANSFORMATION' AND membership_level = 'SCULPT' THEN
E'\u2022 3 sessions per week (12+ sessions per month)\n\u2022 Higher-frequency sculpting with lymphatic drainage built in\n\u2022 Faster contour definition and cellulite improvement\n\u2022 Progress tracking included\n\u2022 Minimum 6-month or 12-month commitment, then month-to-month'

    WHEN membership_name = 'BODY SCULPT & CELLULITE TRANSFORMATION' AND membership_level = 'ELITE SCULPT' THEN
E'\u2022 4 sessions per week (16+ sessions per month)\n\u2022 Maximum sculpting frequency with full lymphatic support\n\u2022 Premium tier for advanced goals and accelerated visual transformation\n\u2022 Progress tracking included\n\u2022 Minimum 6-month or 12-month commitment, then month-to-month'


    WHEN membership_name = 'TONE UP MEMBERSHIP' AND membership_level = 'TONE LITE' THEN
E'\u2022 2 sessions per week across a focused 12-week program\n\u2022 Designed for improved firmness, posture, and body confidence\n\u2022 Gentle and structured \u2014 no gym experience needed\n\u2022 Progress tracking included\n\u2022 Perfect for women wanting to tone without extreme measures'

    WHEN membership_name = 'TONE UP MEMBERSHIP' AND membership_level = 'TONE SCULPT' THEN
E'\u2022 3 sessions per week across a focused 12-week program\n\u2022 Most Popular \u2014 strongest balance of toning frequency and results\n\u2022 Visible firmness and definition improvements throughout the program\n\u2022 Progress tracking included\n\u2022 Built for women ready to see and feel real change'

    WHEN membership_name = 'TONE UP MEMBERSHIP' AND membership_level = 'TONE PRO' THEN
E'\u2022 4 sessions per week across a focused 12-week program\n\u2022 Maximum toning frequency for the fastest visible definition\n\u2022 Premium structure for clients with clear goals and strong commitment\n\u2022 Progress tracking included\n\u2022 Our highest-intensity tone program'

    ELSE inclusions
  END,

  updated_at = NOW()
WHERE tenant_id = (SELECT id FROM tenants WHERE slug = 'cnektp')
  AND deleted_at IS NULL;

SELECT membership_name, membership_level, length(description) AS desc_chars, length(inclusions) AS inc_chars
FROM membership_types
WHERE tenant_id = (SELECT id FROM tenants WHERE slug = 'cnektp')
  AND deleted_at IS NULL
ORDER BY membership_name, membership_level;
