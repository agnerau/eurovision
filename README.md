eurovision

users can connect and create account
with username and password

then they can see main page with all others people stats

then the user can create their own stats in a separate page after pressing a plus sign or smth

they get some layout with all countries participating and then they can maybe drag them to the places in order how they think they will perform

the countries are retrieved from the database and the list is created via separate endpoint for admin (but doesnt need any security checks) and can accept an array of countries, not one by one

if there are for example 25 countries, user doesnt have to list them all - he can just add 5 countries in their places and submit

if user wants to edit places, he sees the same page as in creation just with prefilled places from earlier

and in the end, there should be some kind of podium of who guessed the most right places

the result from actual eurovision could be added also via separate endpoint to WINNER_COUNTRIES table one by one

tables:

USERS

id
username

COUNTRIES

id
name

STATS

id
country
place
user_id

WINNER_COUNTRIES

id
name
place