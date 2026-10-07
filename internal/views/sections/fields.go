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
	f.text("Industry", c.Industry)
	f.add("Company size", c.Size == "", components.Value(c.Size, d.Config.CompanySizes))
	f.text("HQ location", c.HQ)
	f.add("Website", c.Website == "", components.ExtLink(c.Website, c.Website))
	f.add("Notes", c.Notes == "", components.Markdown(c.Notes))
	return f
}

func jobFields(d Data) fields {
	var f fields
	j := d.App.Job
	f.text("Job title", j.Title)
	f.add("Job posting link", j.PostingURL == "", components.ExtLink(j.PostingURL, j.PostingURL))
	f.add("Saved posting copy", j.PostingCopy == "", fileLink(d, j.PostingCopy))
	f.text("Job ID", j.JobID)
	f.text("Department / team", j.Department)
	f.text("Location", j.Location)
	f.add("Key requirements", j.KeyRequirements == "", components.Markdown(j.KeyRequirements))
	f.add("Fit", j.Fit == 0, components.Rating(j.Fit))
	f.add("Interest level", j.Interest == 0, components.Rating(j.Interest))
	f.add("Application channel", j.Channel == "", components.Value(j.Channel, d.Config.Channels))
	f.add("CV version", j.CVVersion == "", cvLink(j.CVVersion, slices.Contains(d.CVs, j.CVVersion)))
	f.add("Extra materials sent", len(j.ExtraMaterials) == 0, materialsList(d))
	f.add("Advertised salary", j.AdvertisedSalary.IsZero(), templ.Raw(templ.EscapeString(j.AdvertisedSalary.String())))
	return f
}

func compensationFields(d Data) fields {
	var f fields
	c := d.App.Compensation
	f.text("Asked salary", c.Asked.String())
	f.text("Offered salary", c.Offered.String())
	f.add("Contract type", c.ContractType == "", components.Value(c.ContractType, d.Config.ContractTypes))
	f.text("Bonus / commission", c.Bonus)
	f.text("Equity", c.Equity)
	f.add("Benefits", c.Benefits == "", components.Markdown(c.Benefits))
	f.text("Notice period", c.NoticePeriod)
	f.add("Start date", c.StartDate.IsZero(), dateValue(c.StartDate, d.Today))
	return f
}

func otherFields(d Data) fields {
	var f fields
	o := d.App.Other
	f.add("Lessons learned", o.LessonsLearned == "", components.Markdown(o.LessonsLearned))
	f.add("Notes", o.Notes == "", components.Markdown(o.Notes))
	return f
}

func processFields(d Data) fields {
	var f fields
	p := d.App.Process
	f.text("Next step", p.NextStep)
	f.add("Next step date", p.NextStepDate.IsZero(), dateValue(p.NextStepDate, d.Today))
	f.add("Last contact", p.LastContactDate.IsZero(), dateValue(p.LastContactDate, d.Today))
	f.add("Date applied", p.DateApplied.IsZero(), dateValue(p.DateApplied, d.Today))
	f.add("Screening", p.ScreeningDate.IsZero(), dateTimeValue(p.ScreeningDate, d.Today))
	f.add("Interviews", len(p.Interviews) == 0, interviewsList(d))
	f.add("Follow-up date", p.FollowUpDate.IsZero(), dateValue(p.FollowUpDate, d.Today))
	f.add("Offer date", p.OfferDate.IsZero(), dateValue(p.OfferDate, d.Today))
	f.add("Take-home task", p.TakeHome.Details == "", components.Markdown(p.TakeHome.Details))
	f.add("Take-home deadline", p.TakeHome.Deadline.IsZero(), dateTimeValue(p.TakeHome.Deadline, d.Today))
	f.add("Decision", p.Decision == "", decisionValue(d))
	f.add("Rejection reason / feedback", p.RejectionFeedback == "", components.Markdown(p.RejectionFeedback))
	f.add("Offer response deadline", p.OfferResponseDeadline.IsZero(), dateValue(p.OfferResponseDeadline, d.Today))
	f.add("Questions & answers", len(p.QA) == 0, qaList(d))
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
