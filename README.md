# transaction_analyzer
Analyzes individual account transaction data from TD Ameritrade to compute statistics about trading performance

# Usage 
- Download the latest binary release to your local environment. 
- Download the transactions data from your TD Ameritrade account (can be found under "My Account" -> "My Account Overview" -> "History & Statememts" -> "Transactions")
- in the same folder where you're running the program, create file named ```config.json``` to
store configurations used at at runtime. 
- set up configurations per your environment.  

## Available configurations
- ```transactionsFile``` the full file path to the transactions csv file that is to be analyzed