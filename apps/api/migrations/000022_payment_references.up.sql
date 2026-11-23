-- References keep only the number; the label (Aut., Ref.) is added when shown.
UPDATE sale_payments SET reference = regexp_replace(reference, '^(Aut\. |TRF )', '') WHERE reference ~ '^(Aut\. |TRF )';
