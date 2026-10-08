package sections

import (
	"slices"

	"github.com/a-h/templ"

	"github.com/G2HJei/job-search-tracker/internal/model"
	"github.com/G2HJei/job-search-tracker/internal/views/components"
)

func companyFields(d Data) fields {
	var f fields
	c := d.Company
	f.text("Industry", "industry", c.Industry)
	f.add("Company size", "size", c.Size == "", components.Value(c.Size, d.Config.CompanySizes))
	f.text("HQ location", "hq", c.HQ)
	f.add("Website", "website", c.Website == "", components.ExtLink(c.Website, c.Website))
	f.add("Notes", "notes", c.Notes == "", components.Markdown(c.Notes))
	return f
}

func jobFields(d Data) fields {
	var f fields
	j := d.App.Job
	f.text("Job title", "title", j.Title)
	f.add("Job posting link", "postingUrl", j.PostingURL == "", components.ExtLink(j.PostingURL, j.PostingURL))
	f.add("Saved posting copy", "postingCopy", j.PostingCopy == "", fileLink(d, j.PostingCopy))
	f.text("Job ID", "jobId", j.JobID)
	f.text("Department / team", "department", j.Department)
	f.text("Location", "location", j.Location)
	f.add("Key requirements", "keyRequirements", j.KeyRequirements == "", components.Markdown(j.KeyRequirements))
	f.add("Fit", "fit", j.Fit == 0, components.Rating(j.Fit))
	f.add("Interest level", "interest", j.Interest == 0, components.Rating(j.Interest))
	f.add("Application channel", "channel", j.Channel == "", components.Value(j.Channel, d.Config.Channels))
	f.add("CV version", "cvVersion", j.CVVersion == "", cvLink(j.CVVersion, slices.Contains(d.CVs, j.CVVersion)))
	f.add("Extra materials sent", "material", len(j.ExtraMaterials) == 0, materialsList(d))
	f.add("Advertised salary", "advertisedSalary_min", j.AdvertisedSalary.IsZero(), templ.Raw(templ.EscapeString(j.AdvertisedSalary.String())))
	return f
}

func compensationFields(d Data) fields {
	var f fields
	c := d.App.Compensation
	f.text("Asked salary", "asked_min", c.Asked.String())
	f.text("Offered salary", "offered_min", c.Offered.String())
	f.add("Contract type", "contractType", c.ContractType == "", components.Value(c.ContractType, d.Config.ContractTypes))
	f.text("Bonus / commission", "bonus", c.Bonus)
	f.text("Equity", "equity", c.Equity)
	f.add("Benefits", "benefits", c.Benefits == "", components.Markdown(c.Benefits))
	f.text("Notice period", "noticePeriod", c.NoticePeriod)
	f.add("Start date", "startDate", c.StartDate.IsZero(), dateValue(c.StartDate, d.Today))
	return f
}

func otherFields(d Data) fields {
	var f fields
	o := d.App.Other
	f.add("Lessons learned", "lessonsLearned", o.LessonsLearned == "", components.Markdown(o.LessonsLearned))
	f.add("Notes", "notes", o.Notes == "", components.Markdown(o.Notes))
	return f
}

func processFields(d Data) fields {
	var f fields
	p := d.App.Process
	f.text("Next step", "nextStep", p.NextStep)
	f.add("Next step date", "nextStepDate", p.NextStepDate.IsZero(), dateValue(p.NextStepDate, d.Today))
	f.add("Last contact", "lastContactDate", p.LastContactDate.IsZero(), dateValue(p.LastContactDate, d.Today))
	f.add("Date applied", "dateApplied", p.DateApplied.IsZero(), dateValue(p.DateApplied, d.Today))
	f.add("Screening", "screeningDate", p.ScreeningDate.IsZero(), dateTimeValue(p.ScreeningDate, d.Today))
	if len(p.Interviews) == 0 { // otherwise shown in a card of their own
		f.add("Interviews", "interview_date", true, nil)
	}
	f.add("Follow-up date", "followUpDate", p.FollowUpDate.IsZero(), dateValue(p.FollowUpDate, d.Today))
	f.add("Offer date", "offerDate", p.OfferDate.IsZero(), dateValue(p.OfferDate, d.Today))
	f.add("Take-home task", "takeHomeDetails", p.TakeHome.Details == "", components.Markdown(p.TakeHome.Details))
	f.add("Take-home deadline", "takeHomeDeadline", p.TakeHome.Deadline.IsZero(), dateTimeValue(p.TakeHome.Deadline, d.Today))
	f.add("Decision", "decision", p.Decision == "", decisionValue(d))
	f.add("Rejection reason / feedback", "rejectionFeedback", p.RejectionFeedback == "", components.Markdown(p.RejectionFeedback))
	f.add("Offer response deadline", "offerResponseDeadline", p.OfferResponseDeadline.IsZero(), dateValue(p.OfferResponseDeadline, d.Today))
	if len(p.QA) == 0 { // otherwise shown in a card of their own
		f.add("Questions & answers", "qa_question", true, nil)
	}
	return f
}

// suggestion returns the status matching the decision when it differs from
// the current status.
func (d Data) suggestion() (model.Status, bool) {
	s, ok := d.Config.SuggestedStatus(d.App.Process.Decision)
	if !ok || s.ID == d.App.Status {
		return model.Status{}, false
	}
	return s, true
}
