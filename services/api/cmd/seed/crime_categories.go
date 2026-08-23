package main

// categoryExcluded marks a recognized primary_type/iucr combination that is
// deliberately not scored (PRD 5.9) -- e.g. gambling, liquor licensing. It
// is a real classification, not "unrecognized": rows classified excluded
// are silently skipped at insert, while genuinely unrecognized combinations
// are logged as a warning -- see classifyCrime.
const categoryExcluded = "excluded"

// crimePrimaryTypeCategory maps every real primary_type value to a
// cpd_crimes category, verified live against the Chicago Data Portal --
// see docs/DATA_SOURCES.md "Crimes" for the full taxonomy and the reasoning
// behind each category (PRD 5.9). OTHER OFFENSE and OFFENSE INVOLVING
// CHILDREN are catch-alls that bundle unrelated iucr sub-codes and are
// deliberately NOT listed here -- see crimeIUCRSubCategory.
var crimePrimaryTypeCategory = map[string]string{
	// Violent (5x)
	"HOMICIDE":                          "violent",
	"ASSAULT":                           "violent",
	"BATTERY":                           "violent",
	"ROBBERY":                           "violent",
	"KIDNAPPING":                        "violent",
	"HUMAN TRAFFICKING":                 "violent",
	"DOMESTIC VIOLENCE":                 "violent", // dead legacy code, 0 records in trailing 12mo
	"WEAPONS VIOLATION":                 "violent",
	"CONCEALED CARRY LICENSE VIOLATION": "violent",
	"STALKING":                          "violent",
	"INTIMIDATION":                      "violent",

	// Sexual (5x)
	"CRIMINAL SEXUAL ASSAULT": "sexual",
	"CRIM SEXUAL ASSAULT":     "sexual", // legacy spelling, same offense -- see docs/DATA_SOURCES.md
	"SEX OFFENSE":             "sexual",

	// Property (1x)
	"THEFT":               "property",
	"BURGLARY":            "property",
	"MOTOR VEHICLE THEFT": "property",
	"CRIMINAL DAMAGE":     "property",
	"ARSON":               "property",
	"CRIMINAL TRESPASS":   "property",

	// Nuisance (2x)
	"NARCOTICS":                        "nuisance",
	"OTHER NARCOTIC VIOLATION":         "nuisance",
	"PROSTITUTION":                     "nuisance",
	"PUBLIC PEACE VIOLATION":           "nuisance",
	"PUBLIC INDECENCY":                 "nuisance",
	"OBSCENITY":                        "nuisance",
	"INTERFERENCE WITH PUBLIC OFFICER": "nuisance",

	// Excluded -- unscored, not a threat-exposure concern for S_safety
	"GAMBLING":             categoryExcluded,
	"NON-CRIMINAL":         categoryExcluded,
	"LIQUOR LAW VIOLATION": categoryExcluded,
	"DECEPTIVE PRACTICE":   categoryExcluded,
	"RITUALISM":            categoryExcluded, // dead legacy code, 0 records in trailing 12mo
}

// crimeIUCRSubCategory maps the individual iucr sub-codes bundled under the
// OTHER OFFENSE and OFFENSE INVOLVING CHILDREN catch-all primary_types.
// Offender-registration-failure codes (5110/5111/5112 Gun, 5130/5131/5132
// Violent, 5121 Arsonist, 4650/4651/4652 Sex) are classified by the
// underlying offense type they register for, not treated as generic
// paperwork violations.
var crimeIUCRSubCategory = map[string]string{
	// Violent
	"1750": "violent", // child abuse
	"1710": "violent", // endangering life/health of child
	"1755": "violent", // child abandonment
	"1790": "violent", // child abduction
	"1780": "violent", // other offense (children)
	"501A": "violent", // animal abuse/neglect
	"500N": "violent", // abuse/neglect - care facility
	"1682": "violent", // animal fighting
	"5000": "violent", // other crime against person
	"2820": "violent", // telephone threat
	"2825": "violent", // harassment by telephone
	"2826": "violent", // harassment by electronic means
	"2830": "violent", // obscene telephone calls
	"4386": "violent", // violation of civil no contact order
	"4387": "violent", // violate order of protection
	"4388": "violent", // violation of bail bond - domestic violence
	"4389": "violent", // violation GPS monitoring device
	"5110": "violent", // gun offender - duty to register
	"5111": "violent", // gun offender - annual registration
	"5112": "violent", // gun offender - duty to report change of information
	"5130": "violent", // violent offender - duty to register
	"5131": "violent", // violent offender - annual registration
	"5132": "violent", // violent offender - fail to register new address
	"5121": "violent", // arsonist - annual registration
	"5007": "violent", // other weapons violation
	"4860": "violent", // board plane with weapon
	"4740": "violent", // unlawful use of body armor

	// Sexual
	"1751": "sexual", // criminal sexual abuse by family member
	"1752": "sexual", // aggravated criminal sexual abuse by family member
	"1753": "sexual", // sexual assault of child by family member
	"1754": "sexual", // aggravated sexual assault of child by family member
	"1582": "sexual", // child pornography
	"1519": "sexual", // distribution photograph depicting sex organs of child
	"1726": "sexual", // minor disseminate indecent depict another minor elec device
	"1537": "sexual", // possession of pornographic print
	"4650": "sexual", // sex offender - fail to register
	"4651": "sexual", // sex offender - fail to register new address
	"4652": "sexual", // sex offender - prohibited zone

	// Property
	"502P": "property", // false/stolen/altered temp plate
	"5002": "property", // other vehicle offense
	"502R": "property", // vehicle title/registration offense
	"502T": "property", // tamper with motor vehicle
	"5001": "property", // other crime involving property
	"4310": "property", // possession of burglary tools
	"5003": "property", // other arson/explosive incident

	// Excluded -- unscored
	"5011": categoryExcluded, // license violation
	"500E": categoryExcluded, // eavesdropping
	"4800": categoryExcluded, // money laundering
	"5009": categoryExcluded, // violation of smoking ban
	"4625": categoryExcluded, // parole violation
	"4510": categoryExcluded, // probation violation
	"5013": categoryExcluded, // violation of summary closure
	"501H": categoryExcluded, // hazardous materials violation
	"1147": categoryExcluded, // internet offenses - electronic fencing
	"1720": categoryExcluded, // contribute to the delinquency of child
	"1725": categoryExcluded, // contribute to the criminal delinquency of child
	"1791": categoryExcluded, // harbor runaway
}

// crimeCatchAllPrimaryTypes are the primary_type values whose real
// classification depends on iucr, not the primary_type itself.
var crimeCatchAllPrimaryTypes = map[string]bool{
	"OTHER OFFENSE":              true,
	"OFFENSE INVOLVING CHILDREN": true,
}

// classifyCrime returns the cpd_crimes category for a given primary_type/
// iucr pair, and false if the combination isn't in the documented taxonomy
// (docs/DATA_SOURCES.md "Crimes") -- a signal to log and skip rather than
// guess, per that doc's own instruction to extend the table when a new
// combination is encountered.
func classifyCrime(primaryType, iucr string) (category string, ok bool) {
	if crimeCatchAllPrimaryTypes[primaryType] {
		category, ok = crimeIUCRSubCategory[iucr]
		return category, ok
	}
	category, ok = crimePrimaryTypeCategory[primaryType]
	return category, ok
}
