## Eurovision predictions!

users can connect and create account with username and password

then they can see main page with all others people stats

then the user can create their own stats in a separate page after pressing a plus sign

they get all countries participating and then they can drag them to the places in order how they think they will perform

the countries are retrieved from the database and the list is created via separate endpoint for admin 

if there are for example 25 countries, user doesnt have to list them all - he can just add 5 countries in their places and submit

if user wants to edit places, he sees the same page as in creation just with prefilled places from earlier

and there is a leaderboard of who guessed the most right places

the result from actual eurovision are added via separate endpoint to WINNER_COUNTRIES table one by one

admin can also lock predictions before results are out

there are two modes - one for semi-final, other for final

admin sets part via endpoint (1,2 or 3)(1,2-semin finals, 3-final)
and based on part results are calculated differently:

    - if it is a semi-final - all countries in top 10 if qualified are marked as correct
    - if it is a final - country and place must match

Tables:

USERS
- id int
- username string

COUNTRIES
- id int
- name string

STATS
- id int
- country_id int
- place int
- user_id int

WINNER_COUNTRIES
- id int
- country_id int
- place int

APP_SETTINGS
- id bool
- predictions_locked bool

EUROVISION_PART
- id bool
- part int