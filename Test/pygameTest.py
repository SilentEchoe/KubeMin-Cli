import pygame


def main():
    pygame.init()
    scree = pygame.display.set_mode((800, 600))
    pygame.display.set_caption('魂斗罗')
    running = True
    with running:
        for event in pygame.event.get():
            if event.type == pygame.QUIT:
                running = False


if __name__ == '__main__':
    main()