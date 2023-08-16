package grammarly_test

import (
	"testing"

	"github.com/buffreak/grammarly-api"
	"github.com/stretchr/testify/assert"
)

func TestLogin(t *testing.T) {
	gws := &grammarly.GrammarlyWS{}
	err := gws.Login("uasgdgjhasbdh@gmail.com", "asdasyu87a")
	assert.Nil(t, err)
}

func TestDoConnection(t *testing.T) {
	gws := &grammarly.GrammarlyWS{}
	err := gws.ConnectWS()
	assert.Nil(t, err)
}

func TestParseResponse(t *testing.T) {
	gws := &grammarly.GrammarlyWS{}
	err := gws.SetCookieFile("cookie.txt")
	assert.Nil(t, err)
	text := `Apply for a personal loan only when you need it. Yes, you read it right. A loan is a financial liability that needs to be paid back. Assess your financial needs and apply for a loan only for important/emergency needs.Calculate your existing payments/EMIs – Take a look at your existing obligations. This may include loan repayments, EMIs, utility bills, credit card bills, etc. If your expense-to-income ratio is already above 30%, there is a high chance that your application for a personal loan will be rejected. It is better to reduce and pay off existing obligations before you apply for a new one.Check your credit score – Having a high credit score tremendously improves your chances of getting a personal loan approved. You can check your credit score for free at CreditMantri. Keeping a check on your credit score will reduce the chances of loan rejection. If your credit score is 750+, go ahead and apply for a personal loan. But if it is less than 700, you might want to improve the score and then apply for a loan. Compare all personal loan offers – Personal loans are offered by banks, NBFCs, fintech lenders, etc. make sure you check the interest rates, loan amount, repayment period, processing charge, and all other charges before you choose to apply for a particular loan. Check for a pre-approved loan offer – Banks offer pre-approved personal loans to their loyal customers with a good credit history. Since the loan is offered by the bank, there is a good chance that the interest rate will be low. Negotiating power of the applicant also increases in such cases. The bank might waive other charges in case of a pre-approved loan.`
	t.Logf("%s\n\n", text)
	// loop 3-5 times to make your content better!
	for i := 0; i < 3; i++ {
		err = gws.ConnectWS()
		assert.Nil(t, err)
		err = gws.WriteRequest(text)
		assert.Nil(t, err)
		text, err = gws.ParseResponse()
		assert.Nil(t, err)
		t.Logf("\n%s\n", text)
	}
}
