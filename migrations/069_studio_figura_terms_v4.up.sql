-- Sample Studio: Insert Membership Agreement v4 and attach to all membership types.
-- Safe to run on any environment; no-ops if the tenant does not exist.

DO $$
DECLARE
    v_tenant_id UUID;
    v_terms_id  UUID;
    v_old_id    UUID;
    v_mt_id     UUID;
BEGIN
    -- Resolve the Sample Studio tenant
    SELECT id INTO v_tenant_id
    FROM tenants
    WHERE slug = 'sample-studio'
       OR name ILIKE '%sample studio%'
    LIMIT 1;

    IF v_tenant_id IS NULL THEN
        RAISE NOTICE 'Sample Studio tenant not found – skipping.';
        RETURN;
    END IF;

    -- Set tenant context for RLS
    PERFORM set_config('app.tenant_id', v_tenant_id::text, true);

    -- Insert the new terms document
    INSERT INTO terms_and_conditions (tenant_id, title, content, version, is_active)
    VALUES (
        v_tenant_id,
        'Membership Agreement & Payment Authority – Version 4',
        E'<h1 style="text-align:center;">SAMPLE STUDIO</h1>\n'
        '<h2 style="text-align:center;">MEMBERSHIP AGREEMENT &amp; PAYMENT AUTHORITY</h2>\n'
        '<p style="text-align:center;">Sample Studio Pty Ltd | ABN 00 000 000 000 | 1 Example Street, Sampletown QLD 4000</p>\n'
        '<hr>\n'

        -- Section A: Member Details
        '<h2>A. MEMBER DETAILS</h2>\n'
        '<table>\n'
        '<tr><td><strong>Full Name:</strong></td><td>{{client_name}}</td></tr>\n'
        '<tr><td><strong>Address:</strong></td><td>{{client_address}}</td></tr>\n'
        '<tr><td><strong>Email:</strong></td><td>{{client_email}}</td></tr>\n'
        '<tr><td><strong>Phone:</strong></td><td>{{client_phone}}</td></tr>\n'
        '<tr><td><strong>Date of Birth:</strong></td><td>{{client_dob}}</td></tr>\n'
        '<tr><td><strong>Gender:</strong></td><td>{{client_gender}}</td></tr>\n'
        '<tr><td><strong>Emergency Contact Name:</strong></td><td>{{emergency_contact_name}}</td></tr>\n'
        '<tr><td><strong>Emergency Contact Phone:</strong></td><td>{{emergency_contact_phone}}</td></tr>\n'
        '</table>\n'

        -- Section B: The Membership
        '<h2>B. THE MEMBERSHIP</h2>\n'
        '<table>\n'
        '<tr><td><strong>Minimum Term:</strong></td><td>{{term_months}} months ({{term_weeks}} weeks)</td></tr>\n'
        '<tr><td><strong>Start Date:</strong></td><td>{{start_date}}</td></tr>\n'
        '<tr><td><strong>Minimum Term End Date:</strong></td><td>{{end_date}}</td></tr>\n'
        '<tr><td><strong>Total Fee for Minimum Term:</strong></td><td>${{total_fee}}</td></tr>\n'
        '<tr><td><strong>Periodic Fee:</strong></td><td>${{payment_amount}} per {{payment_frequency}}</td></tr>\n'
        '<tr><td><strong>Date of First Payment:</strong></td><td>{{start_date}}</td></tr>\n'
        '</table>\n'

        -- Section C: Payment Authority
        '<h2>C. PAYMENT AUTHORITY</h2>\n'
        '<table>\n'
        '<tr><td><strong>Payment Method:</strong></td><td>{{payment_method}}</td></tr>\n'
        '</table>\n'
        '<p>By signing this Agreement, the Member authorises Sample Studio Pty Ltd to charge all Membership Fees and any other amounts payable under this Agreement to the payment method above, via the nominated payment processor, at the frequency and amount specified in Section B, for the full duration of the Minimum Term and thereafter until the membership is cancelled in accordance with this Agreement.</p>\n'
        '<p>This authority is irrevocable for the Minimum Term. The Member must maintain a valid payment method at all times. Failure to do so is a material breach. Cancelling the payment authority does not cancel this Agreement or the obligation to pay. Failed payments incur a $35.00 late fee per occurrence. Payment processor fees apply separately.</p>\n'
        '<p><strong>THIS AGREEMENT IS SUBJECT TO A 7-DAY COOLING-OFF PERIOD FROM THE START DATE. See clause 26 for details.</strong></p>\n'
        '<p><strong>IMPORTANT:</strong> This is a legally binding contract. By signing below you are committing to pay all Membership Fees for the full Minimum Term regardless of whether you attend or use your sessions. Please read the full Agreement terms before signing.</p>\n'
        '<hr>\n'

        -- Full Terms and Conditions
        '<h1 style="text-align:center;">MEMBERSHIP AGREEMENT &ndash; FULL TERMS AND CONDITIONS</h1>\n'
        '<p style="text-align:center;">Sample Studio Pty Ltd | Version 4</p>\n'

        -- PART A
        '<h2>PART A &ndash; PARTIES AND DEFINITIONS</h2>\n'

        '<h3>1. PARTIES</h3>\n'
        '<p>This Agreement is made between Sample Studio Pty Ltd, ABN 00 000 000 000, trading as Sample Studio, 1 Example Street, Sampletown QLD 4000, info@example.com, 0400 000 000 (referred to as &ldquo;Sample Studio&rdquo;, &ldquo;we&rdquo;, &ldquo;us&rdquo; or &ldquo;our&rdquo;) and the individual identified in Section A above (referred to as the &ldquo;Member&rdquo;, &ldquo;you&rdquo; or &ldquo;your&rdquo;).</p>\n'

        '<h3>2. DEFINITIONS</h3>\n'
        '<p>In this Agreement:</p>\n'
        '<p>&ldquo;<strong>Agreement</strong>&rdquo; means this Membership Agreement including the Payment Authority Form, the Summary of Key Terms, and any documents attached or incorporated by reference.</p>\n'
        '<p>&ldquo;<strong>The Membership</strong>&rdquo; means the membership package confirmed by Sample Studio and detailed in Section B above.</p>\n'
        '<p>&ldquo;<strong>Minimum Term</strong>&rdquo; means the fixed period specified in Section B, being {{term_months}} months ({{term_weeks}} weeks), during which the Member is unconditionally obligated to pay all Membership Fees.</p>\n'
        '<p>&ldquo;<strong>Membership Fees</strong>&rdquo; means all amounts payable under this Agreement including periodic membership fees, administration fees, late payment fees, failed payment fees, freeze fees, and any other charges.</p>\n'
        '<p>&ldquo;<strong>Payment Processor</strong>&rdquo; means the third-party payment processing service used by Sample Studio from time to time, which may include but is not limited to Stripe or any successor or replacement provider.</p>\n'
        '<p>&ldquo;<strong>Default</strong>&rdquo; has the meaning given in clause 9.</p>\n'
        '<p>&ldquo;<strong>Remaining Balance</strong>&rdquo; means the total Membership Fees outstanding for the remainder of the Minimum Term at the time of a Default or early cancellation.</p>\n'
        '<p>&ldquo;<strong>Sessions</strong>&rdquo; or &ldquo;<strong>Treatments</strong>&rdquo; means the wellness and body shaping services available to the Member under The Membership.</p>\n'
        '<p>&ldquo;<strong>Freeze</strong>&rdquo; means a temporary suspension of The Membership under clause 27.</p>\n'
        '<p>&ldquo;<strong>Business Day</strong>&rdquo; means a day that is not a Saturday, Sunday, or Queensland public holiday.</p>\n'

        '<h3>3. AGREEMENT OVERVIEW</h3>\n'
        '<p>This Agreement governs the Member''s access to and use of Sample Studio''s wellness and body shaping services under The Membership. By signing the Payment Authority Form, the Member agrees to become a Member of Sample Studio on an ongoing basis subject to the Minimum Term, pay all Membership Fees in full as and when they fall due, comply with all terms and conditions of this Agreement, and abide by Sample Studio''s centre rules and policies as published from time to time.</p>\n'
        '<p>After the Minimum Term expires, The Membership continues automatically as an ongoing agreement until cancelled under clause 21.</p>\n'
        '<p>Sample Studio is part of a network of independently owned and operated wellness and body shaping centres. This Agreement is with Sample Studio Pty Ltd (operator of this centre) only.</p>\n'

        -- PART B
        '<h2>PART B &ndash; PAYMENT OBLIGATIONS</h2>\n'

        '<h3>4. PAYMENT OBLIGATION (ABSOLUTE)</h3>\n'
        '<p>The Member''s obligation to pay Membership Fees is absolute and unconditional. The obligation to pay is not contingent upon and will not be reduced, suspended, or excused by reason of the Member''s failure to attend, access, or use any sessions or treatments; the Member''s dissatisfaction with any service, result, or outcome; any illness, injury, or change in the Member''s circumstances (except as expressly provided in clauses 22 and 23); any dispute between the parties that does not involve a proven breach of this Agreement by Sample Studio; any processing error or outage by the Payment Processor; or any other reason not expressly stated in this Agreement as a ground for cancellation or suspension.</p>\n'
        '<p>The Member agrees that Membership Fees represent fair value for access to the services and that the commitment to pay for the full Minimum Term is a fundamental term upon which Sample Studio relies in operating its business.</p>\n'

        '<h3>5. PAYMENT AUTHORITY</h3>\n'
        '<p>By signing the Payment Authority Form, the Member authorises Sample Studio to charge all Membership Fees and other amounts payable under this Agreement to the nominated payment method via the Payment Processor.</p>\n'
        '<p>This authority is irrevocable during the Minimum Term except as otherwise expressly provided in this Agreement.</p>\n'
        '<p>The Member must maintain a valid, active payment method with sufficient funds at all times. Failure to do so is a material breach of this Agreement.</p>\n'
        '<p>If a payment method expires, is cancelled, or becomes invalid, the Member must immediately notify Sample Studio and provide updated payment details. The obligation to pay continues regardless.</p>\n'
        '<p>Sample Studio may change its Payment Processor at any time. The Member agrees to provide updated payment details or execute any documentation required to facilitate such a change.</p>\n'

        '<h3>6. PAYMENT PROCESSING FEES</h3>\n'
        '<p>The following fees apply in addition to Membership Fees: a failed payment fee of $35.00 per failed or overdue payment, per reattempt; and payment processor fees as published by the Payment Processor from time to time. Sample Studio is not liable for any fee imposed by the Member''s financial institution in connection with a failed payment.</p>\n'

        '<h3>7. NO CHARGEBACK WITHOUT NOTICE</h3>\n'
        '<p>The Member must not initiate any chargeback, payment reversal, or dispute with their bank, credit provider, or the Payment Processor in relation to any payment made or due under this Agreement without first providing Sample Studio with 14 days written notice of the intention to dispute the payment, and allowing Sample Studio a reasonable opportunity to investigate and resolve the dispute within that period.</p>\n'
        '<p>Any chargeback initiated in breach of this clause constitutes a material Default under clause 9 and triggers acceleration of the full Remaining Balance under clause 11.</p>\n'

        '<h3>8. NO SET-OFF</h3>\n'
        '<p>The Member may not withhold, reduce, deduct, or set off any payment against any amount claimed or owing by Sample Studio, whether under this Agreement or otherwise. The payment obligation is independent of any claim the Member may have against Sample Studio.</p>\n'

        '<h3>9. DEFAULT</h3>\n'
        '<p>A Default occurs if the Member fails to make any Membership Fee payment on the due date; cancels, suspends, or interferes with the nominated payment method without complying with this Agreement''s cancellation requirements; initiates a chargeback, reversal, or dispute in breach of clause 7; breaches any payment obligation and fails to remedy that breach within 7 days of written notice from Sample Studio; or breaches any other material term of this Agreement.</p>\n'

        '<h3>10. LATE PAYMENT FEES</h3>\n'
        '<p>Each payment not made on the due date, or each payment attempt that fails for any reason attributable to the Member, including insufficient funds, expired card, or cancellation of payment method, incurs a late fee of $35.00. Each failed reattempt may incur a further $35.00 fee. Late fees are in addition to all other rights of Sample Studio under this Agreement.</p>\n'

        '<h3>11. ACCELERATION ON DEFAULT</h3>\n'
        '<p>Upon a Default, the entire Remaining Balance of Membership Fees for the Minimum Term becomes immediately due and payable in full without further notice. The parties agree that this acceleration clause is a genuine pre-estimate of the loss suffered by Sample Studio as a consequence of the Member''s breach and does not constitute a penalty.</p>\n'

        '<h3>12. INTEREST ON OVERDUE AMOUNTS</h3>\n'
        '<p>Any amount that remains unpaid after its due date accrues interest at 2% per month, calculated daily on the outstanding balance, from the due date until paid in full. Interest accrues whether or not Sample Studio has made a formal demand and is payable in addition to all late fees, recovery costs, and legal fees.</p>\n'

        '<h3>13. RECOVERY COSTS AND INDEMNITY</h3>\n'
        '<p>The Member agrees to pay all costs incurred by Sample Studio in recovering outstanding amounts under this Agreement, including debt collection agency fees, legal fees on a solicitor-and-own-client basis, court costs, process server fees, and administrative costs. The Member indemnifies Sample Studio against all loss, damage, cost, and expense arising from any breach of this Agreement.</p>\n'

        '<h3>14. DEBT COLLECTION AND CREDIT REPORTING</h3>\n'
        '<p>If any amount is overdue, Sample Studio may without further notice refer the debt to a third-party collection agency; report the Default to credit reporting bodies under the Privacy Act 1988 (Cth) and the Australian Privacy Principles; commence legal proceedings in QCAT or any court of competent jurisdiction; and assign, sell, or transfer the debt to a third party.</p>\n'
        '<p>The Member acknowledges that a Default reported to a credit reporting body may adversely affect their credit rating and ability to obtain future credit.</p>\n'

        '<h3>15. ASSIGNMENT OF DEBT</h3>\n'
        '<p>Sample Studio may, without consent of or notice to the Member, assign or transfer any debt or rights under this Agreement to any third party. Following such assignment, the assignee holds the same rights as Sample Studio to recover outstanding amounts.</p>\n'

        '<h3>16. CONTINUING LIABILITY</h3>\n'
        '<p>The Member''s obligation to pay Membership Fees and all other amounts continues notwithstanding the cancellation or expiry of any payment method, the termination of any payment authority, or any purported cancellation of this Agreement that does not comply with the terms of this Agreement. Cancelling a payment method or payment authority does not terminate this Agreement or any payment obligation.</p>\n'

        '<h3>17. PERSONAL LIABILITY</h3>\n'
        '<p>The Member is personally liable for all amounts payable under this Agreement.</p>\n'

        '<h3>18. EVIDENCE OF AMOUNTS OWED</h3>\n'
        '<p>Sample Studio''s records, including electronic records, payment processor records, and accounting records, constitute prima facie evidence of the amounts owed. The Member bears the onus of proving that any amount recorded as owing is incorrect.</p>\n'

        '<h3>19. NO REFUNDS</h3>\n'
        '<p>Membership Fees paid are non-refundable except as required by applicable law including the Australian Consumer Law, or as expressly provided in clause 26. The non-refundable nature of payments was a known and accepted condition of this Agreement.</p>\n'

        '<h3>20. WAIVER OF NOTICE BEFORE ENFORCEMENT</h3>\n'
        '<p>Subject to any express notice requirement in this Agreement, the Member waives any right to receive further notice or demand before Sample Studio takes enforcement action in respect of any Default, including referral to a debt collection agency, credit reporting, or commencement of legal proceedings.</p>\n'

        -- PART C
        '<h2>PART C &ndash; CANCELLATION AND SUSPENSION</h2>\n'

        '<h3>21. CANCELLATION AFTER MINIMUM TERM</h3>\n'
        '<p>After the Minimum Term expires, The Membership continues automatically as an ongoing agreement until cancelled by either party on not less than 14 days written notice by email to the Sample Studio centre address or by written notice delivered to centre staff. The Member remains liable for all Membership Fees due during the notice period.</p>\n'

        '<h3>22. CANCELLATION DURING MINIMUM TERM &ndash; MEDICAL GROUNDS</h3>\n'
        '<p>The Member may apply to cancel during the Minimum Term if they develop a serious illness or suffer a permanent physical incapacity that prevents participation in Sample Studio services. To exercise this right, the Member must submit a written cancellation application to Sample Studio; provide a medical certificate from a registered medical practitioner acceptable to Sample Studio confirming the condition and its permanent or long-term nature; and pay an administration fee equal to 14 days of Membership Fees at the Member''s applicable rate.</p>\n'
        '<p>Sample Studio will assess the application and may approve or decline at its reasonable discretion. A temporary illness or injury expected to resolve does not entitle the Member to cancel. In such circumstances the Member may apply to Freeze The Membership under clause 27.</p>\n'

        '<h3>23. CANCELLATION DURING MINIMUM TERM &ndash; RELOCATION</h3>\n'
        '<p>The Member may cancel during the Minimum Term if they permanently relocate their primary place of residence to a location more than 25 kilometres from any Sample Studio centre, provided they give Sample Studio not less than 30 days written notice; provide satisfactory evidence of relocation such as a rental agreement, sale contract, or statutory declaration; and pay all Membership Fees due up to and including the effective cancellation date. The obligation to pay Membership Fees continues throughout the 30-day notice period regardless of attendance.</p>\n'

        '<h3>24. CANCELLATION DURING MINIMUM TERM &ndash; ALL OTHER REASONS</h3>\n'
        '<p>If the Member wishes to cancel during the Minimum Term for any reason other than those in clauses 22 and 23, the Member may do so by paying a Cancellation Fee equal to 100% of the Remaining Balance of the Minimum Term. The Cancellation Fee is a genuine pre-estimate of Sample Studio''s loss and not a penalty.</p>\n'

        '<h3>25. MISSED SESSIONS AND NON-ATTENDANCE</h3>\n'
        '<p>The Member''s obligation to pay Membership Fees is not affected by a failure to attend, use, or book sessions. Sample Studio is not responsible for monitoring the Member''s usage or for contacting the Member to arrange bookings. Sessions not used in any given week are forfeited without credit, rollover, or compensation.</p>\n'

        '<h3>26. COOLING-OFF PERIOD</h3>\n'
        '<p>The Member may cancel this Agreement within 7 days from the start date by providing written notice to Sample Studio at the centre email address. If the cooling-off right is exercised, Sample Studio will refund amounts paid less any amounts for services already delivered. After the 7-day period, this Agreement is fully binding and the cancellation provisions in clauses 21 to 24 apply exclusively.</p>\n'

        '<h3>27. FREEZE &ndash; SUSPENSION OF MEMBERSHIP</h3>\n'
        '<p>The Member may apply to temporarily freeze The Membership for up to 1 month in any 3-month period. A freeze suspends access to sessions and extends the Minimum Term by the equivalent period.</p>\n'
        '<p>Approved grounds include travel, medical reasons, or financial hardship, with supporting evidence required in each case. A freeze request must be submitted in writing before the intended freeze start date. Sample Studio may approve or decline at its discretion and will not approve a freeze while the Member is in Default.</p>\n'
        '<p>A freeze fee of up to $5.00 per week applies. Additional freeze periods beyond one per 3-month cycle may be approved at Sample Studio''s discretion with supporting documentation.</p>\n'
        '<p>If a government-mandated lockdown or order prevents Sample Studio from operating, The Membership will be automatically frozen for the duration of the closure at no cost. The Minimum Term extends by the equivalent period.</p>\n'

        -- PART D
        '<h2>PART D &ndash; SESSIONS, BOOKINGS AND CONDUCT</h2>\n'

        '<h3>28. SESSIONS AND SERVICES</h3>\n'
        '<p>The Membership provides access to the sessions and treatments specified under The Membership. Services not included in The Membership are available at additional cost and charged separately. Sample Studio reserves the right to modify, replace, or discontinue specific equipment or treatments and will endeavour to provide reasonable advance notice of any material change.</p>\n'

        '<h3>29. BOOKINGS</h3>\n'
        '<p>All sessions and treatments must be pre-booked through Sample Studio''s booking system. The Member is not permitted to attend the centre and use services without a confirmed booking. It is the Member''s sole responsibility to make and manage bookings. Sample Studio is not responsible for monitoring the Member''s usage or reminding the Member to book. Session availability is not guaranteed. Sessions unused in any given week are forfeited without credit, rollover, or compensation.</p>\n'

        '<h3>30. LATE CANCELLATION AND NO-SHOWS</h3>\n'
        '<p>The Member must give a minimum of 24 hours notice to cancel a booked session, by email or by phone during business hours. If the Member cancels with less than 24 hours notice, or fails to attend without notice, the session is forfeited and a late cancellation fee of $50.00 applies, charged to the Member''s nominated payment method.</p>\n'

        '<h3>31. NON-TRANSFERABILITY</h3>\n'
        '<p>The Membership is personal to the Member and cannot be transferred, assigned, shared, or used by any other person.</p>\n'

        '<h3>32. CONDUCT AND CENTRE RULES</h3>\n'
        '<p>The Member agrees to conduct themselves in a respectful and appropriate manner at all times while at the Sample Studio centre and to comply with all centre rules and reasonable staff directions. Sample Studio may terminate The Membership immediately, without refund, where the Member engages in abusive, threatening, or disruptive conduct.</p>\n'

        -- PART E
        '<h2>PART E &ndash; HEALTH, SAFETY AND LIABILITY</h2>\n'

        '<h3>33. HEALTH DECLARATION AND SAFETY</h3>\n'
        '<p>The Member confirms they are in good health, physically capable of participating in Sample Studio''s services, and aware of no medical reason why they should not participate. The Member agrees to provide accurate, complete, and current health information before and during each session, and must inform Sample Studio of any change in health circumstances before their next session. If the Member is uncertain about their fitness to participate, they must seek medical advice before using any Sample Studio service. The Member is responsible for their own safety and welfare while at the centre.</p>\n'

        '<h3>34. LIMITATION OF LIABILITY</h3>\n'
        '<p>To the fullest extent permitted by applicable law, Sample Studio, its directors, officers, employees, agents, and contractors are not liable for any injury including fatal injury, illness, or damage arising from participation in Sample Studio services; any loss, damage, or theft of property on the premises; or any indirect, consequential, or economic loss. Nothing in this clause limits any rights the Member may have under the Australian Consumer Law or other legislation that cannot be excluded by contract.</p>\n'

        '<h3>35. MEMBER INDEMNITY</h3>\n'
        '<p>The Member indemnifies Sample Studio, its directors, officers, employees, and agents from and against all claims, actions, damages, losses, costs, and expenses including legal fees arising from any breach of this Agreement by the Member, any negligence or wilful misconduct of the Member at the centre, or any claim by a third party arising from the Member''s use of the centre.</p>\n'

        -- PART F
        '<h2>PART F &ndash; GENERAL PROVISIONS</h2>\n'

        '<h3>36. PRIVACY</h3>\n'
        '<p>Sample Studio collects, uses, and discloses the Member''s personal information in accordance with the Privacy Act 1988 (Cth) and the Australian Privacy Principles. Personal information may be used for administration of this Agreement, billing and payment processing, debt recovery, disclosure to the Payment Processor and credit reporting bodies as contemplated by this Agreement, and any other purpose required or permitted by law. Sample Studio will not sell the Member''s personal information to third parties for marketing purposes.</p>\n'

        '<h3>37. NO RELIANCE ON REPRESENTATIONS</h3>\n'
        '<p>The Member confirms that in entering into this Agreement they have not relied on any representation, warranty, or statement not expressly contained in this Agreement. This Agreement supersedes all prior discussions, representations, and understandings between the parties.</p>\n'

        '<h3>38. JURISDICTION AND GOVERNING LAW</h3>\n'
        '<p>This Agreement is governed by the laws of Queensland, Australia. Any dispute is subject to the exclusive jurisdiction of the Queensland Civil and Administrative Tribunal (QCAT) or, where the amount exceeds QCAT''s jurisdiction, the courts of Queensland.</p>\n'

        '<h3>39. SEVERABILITY</h3>\n'
        '<p>If any provision of this Agreement is found invalid or unenforceable, it will be severed to the minimum extent necessary and the remainder continues in full force. Where possible, an invalid provision will be read down to preserve the parties'' original intention.</p>\n'

        '<h3>40. SURVIVAL</h3>\n'
        '<p>Clauses relating to payment obligations, Default, interest, acceleration, recovery costs, indemnity, limitation of liability, privacy, credit reporting, and no refunds survive termination or expiry of this Agreement.</p>\n'

        '<h3>41. ENTIRE AGREEMENT</h3>\n'
        '<p>This Agreement, including the Payment Authority Form and any attachments, constitutes the entire agreement between the parties and supersedes all prior negotiations, representations, warranties, and understandings in relation to The Membership.</p>\n'

        -- PART G
        '<h2>PART G &ndash; MEMBER ACKNOWLEDGEMENTS AND SUMMARY</h2>\n'

        '<h3>42. MEMBER ACKNOWLEDGEMENTS</h3>\n'
        '<p>By signing the Payment Authority Form, the Member acknowledges and confirms that they have inspected the Sample Studio centre prior to signing and had the opportunity to ask questions; have read this Agreement in full and understand their obligations including the unconditional commitment to pay Membership Fees for the full Minimum Term; have discussed The Membership with a Sample Studio staff member; understand that cancelling the payment authority or their payment method does not cancel this Agreement or the obligation to pay; are signing this Agreement at the Sample Studio premises; understand that sessions must be pre-booked and that it is solely their responsibility to make bookings; understand that unused sessions are forfeited and do not roll over; acknowledge that The Membership is personal and non-transferable; have provided accurate health information and will seek medical advice if uncertain about fitness to participate; acknowledge that payments are non-refundable except as required by law; understand their 7-day cooling-off rights under clause 26; and have not relied on any statement or representation not contained in this Agreement.</p>\n'

        '<hr>\n'
        '<h2>SUMMARY OF KEY TERMS</h2>\n'
        '<p>The following summary is provided for convenience only and does not limit or replace the full terms above.</p>\n'
        '<ol>\n'
        '<li><strong>Cooling Off:</strong> Cancel within 7 days of start date by written notice to the centre.</li>\n'
        '<li><strong>Minimum Term:</strong> You are committed to pay all Membership Fees for the full Minimum Term. This obligation is absolute and not affected by non-attendance.</li>\n'
        '<li><strong>Payment:</strong> Fees charged via the nominated payment processor. You must maintain a valid payment method at all times.</li>\n'
        '<li><strong>Failed Payment Fee:</strong> $35.00 per failed or overdue payment, per attempt.</li>\n'
        '<li><strong>Default / Acceleration:</strong> On Default, the full Remaining Balance is immediately due.</li>\n'
        '<li><strong>Interest:</strong> Overdue amounts attract 2% per month, calculated daily.</li>\n'
        '<li><strong>Cancellation &ndash; Medical:</strong> Permitted with medical certificate and 14 days administration fee.</li>\n'
        '<li><strong>Cancellation &ndash; Relocation:</strong> 30 days notice required with evidence of move more than 25km from any Sample Studio.</li>\n'
        '<li><strong>Cancellation &ndash; Other:</strong> 100% of the Remaining Balance payable to exit early.</li>\n'
        '<li><strong>After Minimum Term:</strong> The Membership continues until cancelled with 14 days written notice.</li>\n'
        '<li><strong>Freeze:</strong> Once per 3-month period, up to 1 month, up to $5.00 per week fee. Lockdowns auto-freeze at no charge.</li>\n'
        '<li><strong>Bookings:</strong> All sessions must be pre-booked. Sessions do not roll over.</li>\n'
        '<li><strong>Late Cancellation / No-Show:</strong> $50.00 fee. Session is forfeited.</li>\n'
        '<li><strong>Chargeback:</strong> Do not initiate without giving Sample Studio 14 days written notice first.</li>\n'
        '<li><strong>Non-Transferable:</strong> The Membership is personal to you only.</li>\n'
        '<li><strong>Credit Reporting:</strong> Unpaid amounts may be reported to credit reporting bodies.</li>\n'
        '<li><strong>No Refunds:</strong> Payments are non-refundable except as required by law.</li>\n'
        '</ol>\n',
        1,
        true
    )
    RETURNING id INTO v_terms_id;

    RAISE NOTICE 'Created terms_and_conditions id=%', v_terms_id;

    -- Deactivate all previously active T&Cs for this tenant
    UPDATE terms_and_conditions
    SET is_active = false, updated_at = NOW()
    WHERE tenant_id = v_tenant_id
      AND id != v_terms_id
      AND is_active = true;

    -- Detach old T&Cs from every membership type in this tenant
    FOR v_old_id IN
        SELECT DISTINCT terms_and_conditions_id
        FROM membership_type_terms
        WHERE tenant_id = v_tenant_id
          AND terms_and_conditions_id != v_terms_id
    LOOP
        DELETE FROM membership_type_terms
        WHERE tenant_id = v_tenant_id
          AND terms_and_conditions_id = v_old_id;
        RAISE NOTICE 'Detached old terms %', v_old_id;
    END LOOP;

    -- Attach new T&C to every membership type in this tenant
    FOR v_mt_id IN
        SELECT id FROM membership_types WHERE tenant_id = v_tenant_id
    LOOP
        INSERT INTO membership_type_terms (tenant_id, membership_type_id, terms_and_conditions_id)
        VALUES (v_tenant_id, v_mt_id, v_terms_id)
        ON CONFLICT DO NOTHING;
        RAISE NOTICE 'Attached terms to membership_type %', v_mt_id;
    END LOOP;
END $$;
