"""
ManyVids scrapper designed to search for the same videos based on the title,
find the earliest date of release and all links to the same scene
"""
from datetime import datetime
import logging
import requests
from .base_scrapper import BaseScrapper


class ManyVidsScrapper (BaseScrapper):
    """ManyVids scrapper designed to search for the same videos based on the title, 
    find the earliest date of release and all links to the same scene"""

    suggested_date_format = "%b %d, %Y"  # mar 23, 2024
    json_location = "https://www.{domain}/bff/store/videos/{studio_id}/"
    # scene_link = "https://www.{domain}/Scene/{scene_id}/{title_slug}"
    scene_link = "https://www.{domain}/Video/{scene_id}/"

    def get_all_vids(self):
        logging.info("Grabbing all vids from %s", self.studio_link)
        r = requests.get(
            self.json_location.format(
                domain=self.base_domain, studio_id=self.studio_id),
            timeout=self._timeouts["studio"]
        )
        page_number = 1
        logging.info("working on page %d", page_number)
        max_page = int(r.json()['pagination']['totalPages'])
        while page_number < max_page:
            vids_data = r.json()['data']
            self.all_vids.extend(self.to_scenes(vids_data))
            page_number += 1
            logging.info("working on page %d/%d", page_number, max_page)
            p = {"page": page_number}
            r = requests.get(
                self.json_location.format(
                    domain=self.base_domain, studio_id=self.studio_id),
                params=p,
                timeout=self._timeouts["scenes"],
                headers=self._requests_headers
            )
        logging.info("Finished processing all vids from %s", self.studio_link)

    def get_scene_html(self, vid_id):
        r = requests.get(
            self.scene_link.format(domain=self.base_domain, scene_id=vid_id),
            timeout=30,
            headers=self._requests_headers
        )
        if r.status_code != 200:
            raise requests.HTTPError
        return r.text

    def get_title(self, vid_html) -> str:
        parsed_html = super().parse_html(vid_html)
        h1s = parsed_html.find_all("h1", class_="VideoMetaInfo_title__mWRak")
        title = ""
        for h1 in h1s:
            if h1.findChildren():
                continue
            title = h1.text
        logging.debug("Found title, removing whitespaces")
        return " ".join(title.split())

    def get_date_released(self, vid_html) -> str:
        parsed_html = super().parse_html(vid_html)
        date = parsed_html.find("span", class_="VideoDetail_date__Vdd9p").text
        if self.change_date_format == "" or self.change_date_format is None:
            pass
        else:
            try:
                changed_date = datetime.strptime(
                    date,
                    self.suggested_date_format
                ).strftime(self.change_date_format)
                logging.debug("Found the date")
                return changed_date
            except Exception as e:
                raise e
        return date

    def get_details(self, vid_html) -> str:
        parsed_html = super().parse_html(vid_html)
        descr = parsed_html.find(
            "p",
            class_="VideoDetail_partial__T9jkc",
            attrs={"data-testid": "description"}).text
        logging.debug("Found the details")
        return '\n'.join(' '.join(line.split()) for line in descr.split('\n'))

    def get_studio_code(self, vid_html) -> str:
        logging.debug("Found studio_code")
        return [int(i) for i in self.get_url(vid_html).split("/") if i.isdigit()][0]

    def get_duration(self, vid_html):
        parsed_html = super().parse_html(vid_html)
        details_div = parsed_html.find(
            "div", class_="VideoDetail_details__FKwrY")
        duration = details_div.find_all("span")[4].text
        duration = duration.replace("m", ":").replace("h", ":")
        logging.debug("Found duration")
        return duration

    def get_resolution(self, vid_html):
        parsed_html = super().parse_html(vid_html)
        details_div = parsed_html.find(
            "div", class_="VideoDetail_details__FKwrY")
        resolution = details_div.find(
            "span", class_="VideoDetail_quality__J_yq3").text
        logging.debug("Found resolution")
        return resolution.strip()
